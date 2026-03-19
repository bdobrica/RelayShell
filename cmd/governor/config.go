package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/bdobrica/RelayShell/internal/matrixbot"
)

type config struct {
	Matrix           matrixbot.Config
	WorkspaceBaseDir string
	ContainerRuntime string
	ContainerImage   string
	CodexImage       string
	CodexCommand     string
	CopilotImage     string
	CopilotCommand   string
	ContainerEnv     map[string]string
	AllowedUsers     map[string]struct{}
}

func loadConfig() (config, error) {
	homeserver := strings.TrimSpace(os.Getenv("RELAY_MATRIX_HOMESERVER"))
	userID := strings.TrimSpace(os.Getenv("RELAY_MATRIX_USER_ID"))
	accessToken := strings.TrimSpace(os.Getenv("RELAY_MATRIX_ACCESS_TOKEN"))
	governorRoomID := strings.TrimSpace(os.Getenv("RELAY_MATRIX_GOVERNOR_ROOM_ID"))

	if homeserver == "" || userID == "" || accessToken == "" || governorRoomID == "" {
		return config{}, fmt.Errorf("missing required matrix config: RELAY_MATRIX_HOMESERVER, RELAY_MATRIX_USER_ID, RELAY_MATRIX_ACCESS_TOKEN, RELAY_MATRIX_GOVERNOR_ROOM_ID")
	}

	workspaceBaseDir := envWithDefault("RELAY_WORKSPACE_BASE_DIR", "/tmp/relayshell")
	containerRuntime := envWithDefault("RELAY_CONTAINER_RUNTIME", "docker")
	containerImage := envWithDefault("RELAY_CONTAINER_IMAGE", "alpine:3.20")
	codexImage := envWithDefault("RELAY_AGENT_CODEX_IMAGE", "relayshell-codex:latest")
	codexCommand := normalizeCodexCommand(envWithDefault("RELAY_AGENT_CODEX_COMMAND", "codex"))
	copilotImage := envWithDefault("RELAY_AGENT_COPILOT_IMAGE", containerImage)
	copilotCommand := envWithDefault("RELAY_AGENT_COPILOT_COMMAND", "cat")

	passthroughEnv := parseCSVList(envWithDefault(
		"RELAY_CONTAINER_PASSTHROUGH_ENV",
		"OPENAI_API_KEY,OPENAI_BASE_URL,OPENAI_ORG_ID,OPENAI_PROJECT",
	))
	containerEnv := collectProcessEnv(passthroughEnv)

	allowedUsers := parseCSVSet(os.Getenv("RELAY_ALLOWED_USERS"))

	return config{
		Matrix: matrixbot.Config{
			HomeserverURL:  homeserver,
			UserID:         userID,
			AccessToken:    accessToken,
			GovernorRoomID: governorRoomID,
		},
		WorkspaceBaseDir: workspaceBaseDir,
		ContainerRuntime: containerRuntime,
		ContainerImage:   containerImage,
		CodexImage:       codexImage,
		CodexCommand:     codexCommand,
		CopilotImage:     copilotImage,
		CopilotCommand:   copilotCommand,
		ContainerEnv:     containerEnv,
		AllowedUsers:     allowedUsers,
	}, nil
}

func envWithDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func parseCSVSet(value string) map[string]struct{} {
	set := map[string]struct{}{}
	for _, item := range parseCSVList(value) {
		set[item] = struct{}{}
	}
	return set
}

func parseCSVList(value string) []string {
	items := make([]string, 0)
	for _, item := range strings.Split(value, ",") {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		items = append(items, trimmed)
	}
	return items
}

func collectProcessEnv(names []string) map[string]string {
	env := map[string]string{}
	for _, name := range names {
		if value, ok := os.LookupEnv(name); ok && strings.TrimSpace(value) != "" {
			env[name] = value
		}
	}
	return env
}

func normalizeCodexCommand(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == "codex" || strings.HasPrefix(trimmed, "codex ") {
		inner := strings.TrimSpace(strings.TrimPrefix(trimmed, "codex"))
		if inner == "" {
			inner = "--no-alt-screen"
		}
		codexCommand := "codex " + inner
		wrapped := "stty cols 120 rows 40 >/dev/null 2>&1 || true; " + codexCommand
		return fmt.Sprintf("printenv OPENAI_API_KEY | codex login --with-api-key >/dev/null 2>&1 || true; script -q -e -c %q /dev/null", wrapped)
	}
	return trimmed
}
