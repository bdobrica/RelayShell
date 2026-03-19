package gitops

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Manager struct {
	BaseDir string
}

func NewManager(baseDir string) *Manager {
	return &Manager{BaseDir: baseDir}
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

func runGit(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s failed: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}
