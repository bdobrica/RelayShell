package gitops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareUsesSharedMirrorAndSeparateWorktrees(t *testing.T) {
	ctx := context.Background()
	sourceRepo := createSourceRepo(t)
	baseDir := t.TempDir()

	mgr := NewManager(baseDir, "", "")

	workspace1, err := mgr.Prepare(ctx, "sess-1", sourceRepo, "main")
	if err != nil {
		t.Fatalf("Prepare session1 failed: %v", err)
	}
	workspace2, err := mgr.Prepare(ctx, "sess-2", sourceRepo, "main")
	if err != nil {
		t.Fatalf("Prepare session2 failed: %v", err)
	}

	if workspace1 == workspace2 {
		t.Fatalf("expected distinct worktree paths, got %q", workspace1)
	}

	if _, err := os.Stat(filepath.Join(workspace1, "README.md")); err != nil {
		t.Fatalf("workspace1 missing repository content: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace2, "README.md")); err != nil {
		t.Fatalf("workspace2 missing repository content: %v", err)
	}

	repoEntries, err := os.ReadDir(filepath.Join(baseDir, repoCacheDir))
	if err != nil {
		t.Fatalf("ReadDir repo cache failed: %v", err)
	}
	if len(repoEntries) != 1 {
		t.Fatalf("expected one shared mirror, got %d", len(repoEntries))
	}
}

func TestCleanupWorkspaceRemovesWorktreeAndMetadata(t *testing.T) {
	ctx := context.Background()
	sourceRepo := createSourceRepo(t)
	baseDir := t.TempDir()

	mgr := NewManager(baseDir, "", "")
	workspace, err := mgr.Prepare(ctx, "sess-cleanup", sourceRepo, "main")
	if err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}

	if err := mgr.CleanupWorkspace(ctx, workspace); err != nil {
		t.Fatalf("CleanupWorkspace failed: %v", err)
	}

	if _, err := os.Stat(workspace); !os.IsNotExist(err) {
		t.Fatalf("workspace still exists after cleanup: err=%v", err)
	}

	mirrorDir := mgr.mirrorDirForRepo(sourceRepo)
	out, err := runGitOutput(ctx, "--git-dir", mirrorDir, "worktree", "list")
	if err != nil {
		t.Fatalf("worktree list failed: %v", err)
	}
	if strings.Contains(out, workspace) {
		t.Fatalf("worktree metadata still references cleaned workspace: %s", out)
	}
}

func createSourceRepo(t *testing.T) string {
	t.Helper()

	ctx := context.Background()
	repoDir := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir repo dir: %v", err)
	}

	if err := runGit(ctx, "-C", repoDir, "init", "-b", "main"); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := runGit(ctx, "-C", repoDir, "add", "README.md"); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if err := runGit(ctx, "-C", repoDir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "init"); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	return repoDir
}
