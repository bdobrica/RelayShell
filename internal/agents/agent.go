package agents

import (
	"fmt"
	"strings"
)

type Spec struct {
	Image   string
	Command string
}

type Resolver struct {
	DefaultImage   string
	CodexImage     string
	CodexCommand   string
	CopilotImage   string
	CopilotCommand string
}

func (r Resolver) Resolve(agent string) (Spec, error) {
	switch strings.ToLower(strings.TrimSpace(agent)) {
	case "codex":
		return Spec{
			Image:   coalesce(r.CodexImage, r.DefaultImage),
			Command: coalesce(r.CodexCommand, "codex"),
		}, nil
	case "copilot":
		return Spec{
			Image:   coalesce(r.CopilotImage, r.DefaultImage),
			Command: coalesce(r.CopilotCommand, "cat"),
		}, nil
	default:
		return Spec{}, fmt.Errorf("unsupported agent %q", agent)
	}
}

func coalesce(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
