package agents

import (
	"fmt"
	"strings"
)

// CommandFor returns the shell command that should be executed in the agent container.
func CommandFor(agent string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(agent)) {
	case "codex", "copilot":
		// Phase 1 uses a simple echo-like process to validate end-to-end bridging.
		return "cat", nil
	default:
		return "", fmt.Errorf("unsupported agent %q", agent)
	}
}
