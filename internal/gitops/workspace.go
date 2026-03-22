package gitops

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

var ErrNoChanges = errors.New("no changes to commit")

const (
	worktreeDirName = "worktrees"
	repoCacheDir    = "repos"
)

var nonAlnumDashUnderscore = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

type CommitResult struct {
	SHA     string
	Message string
	Files   []string
}

type DiffFileSummary struct {
	Path    string
	Added   int
	Removed int
}

type PushOptions struct {
	Remote        string
	Branch        string
	SSHKeyPath    string
	SSHPrivateKey string
}

type Manager struct {
	BaseDir          string
	CommitAuthorName string
	CommitAuthorMail string
	mu               sync.Mutex
}

func NewManager(baseDir, commitAuthorName, commitAuthorEmail string) *Manager {
	return &Manager{
		BaseDir:          baseDir,
		CommitAuthorName: strings.TrimSpace(commitAuthorName),
		CommitAuthorMail: strings.TrimSpace(commitAuthorEmail),
	}
}

// Prepare ensures a shared bare mirror and creates a per-session worktree.
func (m *Manager) Prepare(ctx context.Context, sessionID, repo, branch string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := os.MkdirAll(m.BaseDir, 0o755); err != nil {
		return "", fmt.Errorf("create workspace base dir: %w", err)
	}
	if err := os.MkdirAll(m.reposBaseDir(), 0o755); err != nil {
		return "", fmt.Errorf("create repo cache dir: %w", err)
	}
	if err := os.MkdirAll(m.worktreesBaseDir(), 0o755); err != nil {
		return "", fmt.Errorf("create worktrees dir: %w", err)
	}

	repoPath := strings.TrimSpace(repo)
	if repoPath == "" {
		return "", fmt.Errorf("repo is required")
	}
	branchName := strings.TrimSpace(branch)
	if branchName == "" {
		return "", fmt.Errorf("branch is required")
	}

	if err := m.ensureMirror(ctx, repoPath); err != nil {
		return "", err
	}

	workspaceDir := filepath.Join(m.worktreesBaseDir(), sessionID)
	if err := m.cleanupWorkspaceLocked(ctx, workspaceDir); err != nil {
		return "", fmt.Errorf("cleanup existing workspace: %w", err)
	}

	mirrorDir := m.mirrorDirForRepo(repoPath)
	branchRef, err := m.resolveBranchRef(ctx, mirrorDir, branchName)
	if err != nil {
		return "", err
	}

	worktreeBranch := m.worktreeBranchName(sessionID, branchName)
	if err := runGit(
		ctx,
		"--git-dir", mirrorDir,
		"worktree", "add",
		"-B", worktreeBranch,
		workspaceDir,
		branchRef,
	); err != nil {
		return "", err
	}

	if err := runGit(ctx, "-C", workspaceDir, "branch", "--set-upstream-to=origin/"+branchName, worktreeBranch); err != nil {
		// Upstream setup can fail for unusual branch layouts; it is non-fatal for local commits.
	}

	return workspaceDir, nil
}

// CleanupWorkspace removes a session worktree and prunes mirror metadata.
func (m *Manager) CleanupWorkspace(ctx context.Context, workspaceDir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.cleanupWorkspaceLocked(ctx, workspaceDir)
}

func (m *Manager) cleanupWorkspaceLocked(ctx context.Context, workspaceDir string) error {

	trimmedWorkspace := strings.TrimSpace(workspaceDir)
	if trimmedWorkspace == "" {
		return nil
	}

	if stat, err := os.Stat(trimmedWorkspace); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat workspace: %w", err)
	} else if !stat.IsDir() {
		if err := os.Remove(trimmedWorkspace); err != nil {
			return fmt.Errorf("remove non-directory workspace: %w", err)
		}
		return nil
	}

	mirrorDir, err := resolveMirrorDirFromWorktree(trimmedWorkspace)
	if err == nil && mirrorDir != "" {
		_ = runGit(ctx, "--git-dir", mirrorDir, "worktree", "remove", "--force", trimmedWorkspace)
		_ = runGit(ctx, "--git-dir", mirrorDir, "worktree", "prune")
	}

	if err := os.RemoveAll(trimmedWorkspace); err != nil {
		return fmt.Errorf("remove workspace directory: %w", err)
	}

	return nil
}

func (m *Manager) CommitAll(ctx context.Context, workspaceDir string) (CommitResult, error) {
	if err := runGit(ctx, "-C", workspaceDir, "add", "-A"); err != nil {
		return CommitResult{}, err
	}

	stagedOutput, err := runGitOutput(ctx, "-C", workspaceDir, "diff", "--cached", "--name-only")
	if err != nil {
		return CommitResult{}, err
	}

	files := splitNonEmptyLines(stagedOutput)
	if len(files) == 0 {
		return CommitResult{}, ErrNoChanges
	}

	message := buildCommitMessage(files)
	authorName, authorEmail := m.resolveCommitIdentity(ctx)
	if err := runGit(
		ctx,
		"-C", workspaceDir,
		"-c", "user.name="+authorName,
		"-c", "user.email="+authorEmail,
		"commit",
		"-m", message,
	); err != nil {
		return CommitResult{}, err
	}

	shaOutput, err := runGitOutput(ctx, "-C", workspaceDir, "rev-parse", "--short", "HEAD")
	if err != nil {
		return CommitResult{}, err
	}

	return CommitResult{
		SHA:     strings.TrimSpace(shaOutput),
		Message: message,
		Files:   files,
	}, nil
}

func (m *Manager) WorkspaceTree(workspaceDir string, maxDepth, maxEntries int) (string, error) {
	root := strings.TrimSpace(workspaceDir)
	if root == "" {
		return "", fmt.Errorf("workspace directory is required")
	}

	if maxDepth <= 0 {
		maxDepth = 5
	}
	if maxEntries <= 0 {
		maxEntries = 400
	}

	if stat, err := os.Stat(root); err != nil {
		return "", fmt.Errorf("stat workspace: %w", err)
	} else if !stat.IsDir() {
		return "", fmt.Errorf("workspace path is not a directory")
	}

	var output strings.Builder
	output.WriteString(".\n")

	entriesCount := 0
	truncated := false

	var walk func(dir, prefix string, depth int) error
	walk = func(dir, prefix string, depth int) error {
		if depth >= maxDepth || truncated {
			return nil
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}

		filtered := make([]os.DirEntry, 0, len(entries))
		for _, entry := range entries {
			if entry.Name() == ".git" {
				continue
			}
			filtered = append(filtered, entry)
		}

		for idx, entry := range filtered {
			if entriesCount >= maxEntries {
				truncated = true
				return nil
			}

			entriesCount++
			isLast := idx == len(filtered)-1
			connector := "|-- "
			nextPrefix := prefix + "|   "
			if isLast {
				connector = "`-- "
				nextPrefix = prefix + "    "
			}

			name := entry.Name()
			if entry.IsDir() {
				name += "/"
			}
			output.WriteString(prefix + connector + name + "\n")

			if entry.IsDir() {
				if err := walk(filepath.Join(dir, entry.Name()), nextPrefix, depth+1); err != nil {
					return err
				}
			}
		}

		return nil
	}

	if err := walk(root, "", 0); err != nil {
		return "", fmt.Errorf("build workspace tree: %w", err)
	}

	if truncated {
		output.WriteString(fmt.Sprintf("... truncated after %d entries\n", maxEntries))
	}

	return output.String(), nil
}

func (m *Manager) DiffSummary(ctx context.Context, workspaceDir string) ([]DiffFileSummary, error) {
	out, err := runGitOutput(ctx, "-C", workspaceDir, "diff", "--numstat", "HEAD")
	if err != nil {
		return nil, err
	}

	byPath := map[string]DiffFileSummary{}
	for _, line := range splitNonEmptyLines(out) {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}

		added, _ := strconv.Atoi(parts[0])
		removed, _ := strconv.Atoi(parts[1])
		path := strings.TrimSpace(parts[2])
		if path == "" {
			continue
		}

		byPath[path] = DiffFileSummary{Path: path, Added: added, Removed: removed}
	}

	untracked, err := runGitOutput(ctx, "-C", workspaceDir, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	for _, path := range splitNonEmptyLines(untracked) {
		if _, exists := byPath[path]; exists {
			continue
		}
		byPath[path] = DiffFileSummary{Path: path, Added: 0, Removed: 0}
	}

	summaries := make([]DiffFileSummary, 0, len(byPath))
	for _, item := range byPath {
		summaries = append(summaries, item)
	}
	sort.Slice(summaries, func(i, j int) bool { return summaries[i].Path < summaries[j].Path })

	return summaries, nil
}

func (m *Manager) DiffFile(ctx context.Context, workspaceDir, relativePath string) (string, error) {
	trimmed := strings.TrimSpace(relativePath)
	if trimmed == "" {
		return "", fmt.Errorf("file path is required")
	}
	if filepath.IsAbs(trimmed) {
		return "", fmt.Errorf("file path must be relative")
	}
	if strings.HasPrefix(filepath.Clean(trimmed), "..") {
		return "", fmt.Errorf("file path must stay inside workspace")
	}

	out, err := runGitOutput(ctx, "-C", workspaceDir, "diff", "HEAD", "--", trimmed)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(out) != "" {
		return out, nil
	}

	untracked, err := runGitOutput(ctx, "-C", workspaceDir, "ls-files", "--others", "--exclude-standard", "--", trimmed)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(untracked) == "" {
		return out, nil
	}

	noIndexOut, err := runGitOutputAllowExitCode(ctx, []int{0, 1}, "-C", workspaceDir, "diff", "--no-index", "--", "/dev/null", trimmed)
	if err != nil {
		return "", err
	}

	return noIndexOut, nil
}

func (m *Manager) Push(ctx context.Context, workspaceDir string, options PushOptions) (string, error) {
	sshKeyPath := strings.TrimSpace(options.SSHKeyPath)
	privateKey := normalizePrivateKey(options.SSHPrivateKey)

	if sshKeyPath == "" && privateKey == "" {
		return "", fmt.Errorf("push requires SSH key configuration")
	}

	if sshKeyPath == "" {
		keyDir := filepath.Join(workspaceDir, ".relayshell")
		if err := os.MkdirAll(keyDir, 0o700); err != nil {
			return "", fmt.Errorf("create key directory: %w", err)
		}

		tmpFile, err := os.CreateTemp(keyDir, "push-key-*")
		if err != nil {
			return "", fmt.Errorf("create temp ssh key file: %w", err)
		}
		tmpPath := tmpFile.Name()
		if _, err := tmpFile.WriteString(privateKey); err != nil {
			_ = tmpFile.Close()
			_ = os.Remove(tmpPath)
			return "", fmt.Errorf("write temp ssh key file: %w", err)
		}
		if err := tmpFile.Close(); err != nil {
			_ = os.Remove(tmpPath)
			return "", fmt.Errorf("close temp ssh key file: %w", err)
		}
		if err := os.Chmod(tmpPath, 0o600); err != nil {
			_ = os.Remove(tmpPath)
			return "", fmt.Errorf("chmod temp ssh key file: %w", err)
		}

		defer os.Remove(tmpPath)
		sshKeyPath = tmpPath
	}

	remote := strings.TrimSpace(options.Remote)
	if remote == "" {
		remote = "origin"
	}

	branch := strings.TrimSpace(options.Branch)
	if branch == "" {
		branchOut, err := runGitOutput(ctx, "-C", workspaceDir, "rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			return "", err
		}
		branch = strings.TrimSpace(branchOut)
	}

	sshCommand := fmt.Sprintf("ssh -i %s -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new", shellEscape(sshKeyPath))
	cmd := exec.CommandContext(ctx, "git", "-C", workspaceDir, "push", remote, branch)
	cmd.Env = append(os.Environ(), "GIT_SSH_COMMAND="+sshCommand)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git push failed: %w: %s", err, strings.TrimSpace(string(output)))
	}

	return string(output), nil
}

func normalizePrivateKey(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	return strings.ReplaceAll(trimmed, `\n`, "\n") + "\n"
}

func shellEscape(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func buildCommitMessage(files []string) string {
	if len(files) == 1 {
		return "Update " + files[0]
	}
	if len(files) == 2 {
		return "Update " + files[0] + " and " + files[1]
	}
	return fmt.Sprintf("Update %d files", len(files))
}

func splitNonEmptyLines(value string) []string {
	lines := strings.Split(value, "\n")
	items := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		items = append(items, trimmed)
	}
	return items
}

func (m *Manager) resolveCommitIdentity(ctx context.Context) (string, string) {
	name := m.CommitAuthorName
	email := m.CommitAuthorMail

	if name == "" {
		if value, ok := globalGitConfig(ctx, "user.name"); ok {
			name = value
		}
	}
	if email == "" {
		if value, ok := globalGitConfig(ctx, "user.email"); ok {
			email = value
		}
	}

	if name == "" {
		name = "RelayShell"
	}
	if email == "" {
		email = "relayshell@local"
	}

	return name, email
}

func globalGitConfig(ctx context.Context, key string) (string, bool) {
	output, err := runGitOutput(ctx, "config", "--global", "--get", key)
	if err != nil {
		return "", false
	}
	value := strings.TrimSpace(output)
	if value == "" {
		return "", false
	}
	return value, true
}

func runGit(ctx context.Context, args ...string) error {
	_, err := runGitOutput(ctx, args...)
	return err
}

func runGitOutput(ctx context.Context, args ...string) (string, error) {
	out, err := runGitOutputAllowExitCode(ctx, []int{0}, args...)
	if err != nil {
		return "", err
	}
	return out, nil
}

func runGitOutputAllowExitCode(ctx context.Context, allowedExitCodes []int, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return string(output), nil
	}

	exitCode := -1
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		exitCode = exitErr.ExitCode()
	}

	for _, allowed := range allowedExitCodes {
		if allowed == exitCode {
			return string(output), nil
		}
	}

	return "", fmt.Errorf("git %s failed: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
}

func (m *Manager) ensureMirror(ctx context.Context, repo string) error {
	mirrorDir := m.mirrorDirForRepo(repo)
	if _, err := os.Stat(mirrorDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := runGit(ctx, "clone", "--mirror", repo, mirrorDir); err != nil {
				return err
			}
			return nil
		}
		return fmt.Errorf("stat mirror dir: %w", err)
	}

	if err := runGit(ctx, "--git-dir", mirrorDir, "remote", "set-url", "origin", repo); err != nil {
		return err
	}
	if err := runGit(ctx, "--git-dir", mirrorDir, "fetch", "--prune", "origin"); err != nil {
		return err
	}
	return nil
}

func (m *Manager) resolveBranchRef(ctx context.Context, mirrorDir, branch string) (string, error) {
	remoteRef := "refs/remotes/origin/" + branch
	if err := runGit(ctx, "--git-dir", mirrorDir, "show-ref", "--verify", "--quiet", remoteRef); err == nil {
		return remoteRef, nil
	}

	localRef := "refs/heads/" + branch
	if err := runGit(ctx, "--git-dir", mirrorDir, "show-ref", "--verify", "--quiet", localRef); err == nil {
		return localRef, nil
	}

	return "", fmt.Errorf("branch %q not found in repository %q", branch, mirrorDir)
}

func (m *Manager) worktreesBaseDir() string {
	return filepath.Join(m.BaseDir, worktreeDirName)
}

func (m *Manager) reposBaseDir() string {
	return filepath.Join(m.BaseDir, repoCacheDir)
}

func (m *Manager) mirrorDirForRepo(repo string) string {
	trimmed := strings.TrimSpace(repo)
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(trimmed)))
	name := sanitizedRepoName(trimmed)
	if name == "" {
		name = "repo"
	}
	return filepath.Join(m.reposBaseDir(), name+"-"+hash[:12]+".git")
}

func sanitizedRepoName(repo string) string {
	base := strings.TrimSuffix(filepath.Base(strings.TrimSpace(repo)), ".git")
	base = nonAlnumDashUnderscore.ReplaceAllString(base, "-")
	base = strings.Trim(base, "-")
	if base == "" {
		return "repo"
	}
	return base
}

func (m *Manager) worktreeBranchName(sessionID, branch string) string {
	sessionPart := strings.TrimSpace(sessionID)
	if sessionPart == "" {
		sessionPart = "session"
	}
	branchPart := nonAlnumDashUnderscore.ReplaceAllString(strings.TrimSpace(branch), "-")
	branchPart = strings.Trim(branchPart, "-")
	if branchPart == "" {
		branchPart = "branch"
	}
	return "relayshell/" + sessionPart + "/" + branchPart
}

func resolveMirrorDirFromWorktree(workspaceDir string) (string, error) {
	gitFile := filepath.Join(workspaceDir, ".git")
	data, err := os.ReadFile(gitFile)
	if err != nil {
		return "", fmt.Errorf("read .git file: %w", err)
	}

	content := strings.TrimSpace(string(data))
	const prefix = "gitdir:"
	if !strings.HasPrefix(content, prefix) {
		return "", fmt.Errorf("workspace .git is not a linked worktree")
	}

	gitDirPath := strings.TrimSpace(strings.TrimPrefix(content, prefix))
	if gitDirPath == "" {
		return "", fmt.Errorf("workspace .git file has empty gitdir")
	}
	if !filepath.IsAbs(gitDirPath) {
		gitDirPath = filepath.Join(workspaceDir, gitDirPath)
	}
	cleanGitDir := filepath.Clean(gitDirPath)
	marker := string(filepath.Separator) + "worktrees" + string(filepath.Separator)
	idx := strings.LastIndex(cleanGitDir, marker)
	if idx < 0 {
		return "", fmt.Errorf("unable to resolve mirror dir from gitdir path")
	}

	return cleanGitDir[:idx], nil
}
