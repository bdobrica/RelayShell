package gitops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var ErrNoChanges = errors.New("no changes to commit")

type CommitResult struct {
	SHA     string
	Message string
	Files   []string
}

type Manager struct {
	BaseDir          string
	CommitAuthorName string
	CommitAuthorMail string
}

func NewManager(baseDir, commitAuthorName, commitAuthorEmail string) *Manager {
	return &Manager{
		BaseDir:          baseDir,
		CommitAuthorName: strings.TrimSpace(commitAuthorName),
		CommitAuthorMail: strings.TrimSpace(commitAuthorEmail),
	}
}

// Prepare clones a repository and checks out the requested branch.
func (m *Manager) Prepare(ctx context.Context, sessionID, repo, branch string) (string, error) {
	if err := os.MkdirAll(m.BaseDir, 0o755); err != nil {
		return "", fmt.Errorf("create workspace base dir: %w", err)
	}

	workspaceDir := filepath.Join(m.BaseDir, sessionID)
	if err := os.RemoveAll(workspaceDir); err != nil {
		return "", fmt.Errorf("cleanup existing workspace: %w", err)
	}

	if err := runGit(ctx, "clone", repo, workspaceDir); err != nil {
		return "", err
	}
	if err := runGit(ctx, "-C", workspaceDir, "checkout", branch); err != nil {
		return "", err
	}

	return workspaceDir, nil
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
