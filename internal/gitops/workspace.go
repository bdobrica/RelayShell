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
	cmd := exec.CommandContext(ctx, "git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
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
