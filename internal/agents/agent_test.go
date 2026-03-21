package agents

import (
	"strings"
	"testing"
)

func TestResolverResolveCodex(t *testing.T) {
	r := NewResolver(BackendOptions{
		DefaultImage:   "alpine:3.20",
		CodexImage:     "relayshell-codex:latest",
		CodexCommand:   "codex --no-alt-screen",
		CopilotImage:   "relayshell-copilot:latest",
		CopilotCommand: "copilot",
	})

	spec, err := r.Resolve("codex")
	if err != nil {
		t.Fatalf("Resolve(codex) error = %v", err)
	}

	if spec.Image != "relayshell-codex:latest" {
		t.Fatalf("Image = %q, want relayshell-codex:latest", spec.Image)
	}
	if spec.Command != "codex --no-alt-screen" {
		t.Fatalf("Command = %q, want codex --no-alt-screen", spec.Command)
	}
}

func TestResolverResolveCopilot(t *testing.T) {
	r := NewResolver(BackendOptions{
		DefaultImage: "alpine:3.20",
	})

	spec, err := r.Resolve("copilot")
	if err != nil {
		t.Fatalf("Resolve(copilot) error = %v", err)
	}

	if spec.Image != "alpine:3.20" {
		t.Fatalf("Image = %q, want alpine:3.20", spec.Image)
	}
	if spec.Command != "copilot" {
		t.Fatalf("Command = %q, want copilot", spec.Command)
	}
}

func TestResolverResolveUnsupported(t *testing.T) {
	r := NewResolver(BackendOptions{DefaultImage: "alpine:3.20"})

	_, err := r.Resolve("unknown")
	if err == nil {
		t.Fatal("Resolve(unknown) expected error")
	}
	if !strings.Contains(err.Error(), "unsupported agent") {
		t.Fatalf("error = %q, want unsupported agent", err.Error())
	}
}
