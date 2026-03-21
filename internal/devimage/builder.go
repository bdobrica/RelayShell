package devimage

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var nonTagChars = regexp.MustCompile(`[^a-z0-9_.-]+`)

func BuildDerivedImage(ctx context.Context, runtime, workspaceDir, sessionID string, stack Stack, timeout time.Duration) (string, error) {
	if strings.TrimSpace(runtime) == "" {
		return "", fmt.Errorf("container runtime is required")
	}
	if strings.TrimSpace(workspaceDir) == "" {
		return "", fmt.Errorf("workspace dir is required")
	}

	dir := filepath.Join(workspaceDir, ".relayshell")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create relayshell build dir: %w", err)
	}

	dockerfilePath := filepath.Join(dir, "Dockerfile.dev")
	content := RenderDockerfile()
	if err := os.WriteFile(dockerfilePath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write generated Dockerfile: %w", err)
	}

	tag := derivedImageTag(sessionID, stack)
	buildCtx := ctx
	cancel := func() {}
	if timeout > 0 {
		buildCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	buildArgs := []string{"build", "-f", dockerfilePath, "-t", tag}
	buildArgs = append(buildArgs, buildArgsForStack(stack)...)
	buildArgs = append(buildArgs, workspaceDir)

	cmd := exec.CommandContext(buildCtx, runtime, buildArgs...)
	cmd.Env = append(os.Environ(), "DOCKER_BUILDKIT=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("build derived image failed: %w: %s", err, strings.TrimSpace(string(output)))
	}

	return tag, nil
}

func derivedImageTag(sessionID string, stack Stack) string {
	sid := strings.ToLower(strings.TrimSpace(sessionID))
	if sid == "" {
		sid = "session"
	}
	sid = nonTagChars.ReplaceAllString(sid, "-")
	sid = strings.Trim(sid, "-.")
	if sid == "" {
		sid = "session"
	}

	stackPart := strings.ToLower(string(stack))
	if stackPart == "" {
		stackPart = "unknown"
	}

	return fmt.Sprintf("relayshell-dev-%s-%s", sid, stackPart)
}
