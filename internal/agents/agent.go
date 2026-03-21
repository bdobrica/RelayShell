package agents

import (
	"fmt"
	"strings"
)

type Spec struct {
	Image   string
	Command string
}

type AgentBackend interface {
	Name() string
	Spec() (Spec, error)
}

type BackendOptions struct {
	DefaultImage   string
	CodexImage     string
	CodexCommand   string
	CopilotImage   string
	CopilotCommand string
}

type Resolver struct {
	backends map[string]AgentBackend
}

func NewResolver(options BackendOptions) Resolver {
	return Resolver{
		backends: map[string]AgentBackend{
			"codex": codexBackend{
				defaultImage: options.DefaultImage,
				image:        options.CodexImage,
				command:      options.CodexCommand,
			},
			"copilot": copilotBackend{
				defaultImage: options.DefaultImage,
				image:        options.CopilotImage,
				command:      options.CopilotCommand,
			},
		},
	}
}

func (r Resolver) Resolve(agent string) (Spec, error) {
	name := strings.ToLower(strings.TrimSpace(agent))
	backend, ok := r.backends[name]
	if !ok {
		return Spec{}, fmt.Errorf("unsupported agent %q", agent)
	}

	return backend.Spec()
}

type codexBackend struct {
	defaultImage string
	image        string
	command      string
}

func (b codexBackend) Name() string { return "codex" }

func (b codexBackend) Spec() (Spec, error) {
	return resolveSpec(b.defaultImage, b.image, b.command, "codex")
}

type copilotBackend struct {
	defaultImage string
	image        string
	command      string
}

func (b copilotBackend) Name() string { return "copilot" }

func (b copilotBackend) Spec() (Spec, error) {
	return resolveSpec(b.defaultImage, b.image, b.command, "copilot")
}

func resolveSpec(defaultImage, configuredImage, configuredCommand, fallbackCommand string) (Spec, error) {
	image := coalesce(configuredImage, defaultImage)
	if strings.TrimSpace(image) == "" {
		return Spec{}, fmt.Errorf("agent image is required")
	}

	command := coalesce(configuredCommand, fallbackCommand)
	if strings.TrimSpace(command) == "" {
		return Spec{}, fmt.Errorf("agent command is required")
	}

	return Spec{Image: image, Command: command}, nil
}

func coalesce(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
