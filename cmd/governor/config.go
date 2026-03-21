package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bdobrica/RelayShell/internal/matrixbot"
)

type config struct {
	Matrix              matrixbot.Config
	WorkspaceBaseDir    string
	EventsDBPath        string
	EventsRetentionDays int
	ContainerRuntime    string
	ContainerImage      string
	CodexImage          string
	CodexCommand        string
	CopilotImage        string
	CopilotCommand      string
	ContainerEnv        map[string]string
	GitAuthorName       string
	GitAuthorEmail      string
	AllowedUsers        map[string]struct{}
	BridgeBatchIdle     time.Duration
	BridgeFlushMax      time.Duration
	BridgeDebugIO       bool
	RoomArchivePolicy   roomArchivePolicy
	DevImageTemplates   bool
	DevImageBuildTO     time.Duration
}

type roomArchivePolicy string

const (
	roomArchiveKeep   roomArchivePolicy = "keep"
	roomArchiveLeave  roomArchivePolicy = "leave"
	roomArchiveForget roomArchivePolicy = "forget"
)

func loadConfig() (config, error) {
	homeserver := strings.TrimSpace(os.Getenv("RELAY_MATRIX_HOMESERVER"))
	userID := strings.TrimSpace(os.Getenv("RELAY_MATRIX_USER_ID"))
	accessToken := strings.TrimSpace(os.Getenv("RELAY_MATRIX_ACCESS_TOKEN"))
	governorRoomID := strings.TrimSpace(os.Getenv("RELAY_MATRIX_GOVERNOR_ROOM_ID"))

	if homeserver == "" || userID == "" || accessToken == "" || governorRoomID == "" {
		return config{}, fmt.Errorf("missing required matrix config: RELAY_MATRIX_HOMESERVER, RELAY_MATRIX_USER_ID, RELAY_MATRIX_ACCESS_TOKEN, RELAY_MATRIX_GOVERNOR_ROOM_ID")
	}

	workspaceBaseDir := envWithDefault("RELAY_WORKSPACE_BASE_DIR", "/tmp/relayshell")
	eventsDBPath := envWithDefault("RELAY_EVENTS_DB_PATH", workspaceBaseDir+"/governor_events.db")
	eventsRetentionDays, err := envNonNegativeInt("RELAY_EVENTS_RETENTION_DAYS", 30)
	if err != nil {
		return config{}, err
	}
	containerRuntime := envWithDefault("RELAY_CONTAINER_RUNTIME", "docker")
	containerImage := envWithDefault("RELAY_CONTAINER_IMAGE", "alpine:3.20")
	codexImage := envWithDefault("RELAY_AGENT_CODEX_IMAGE", "relayshell-codex:latest")
	codexCommand := normalizeCodexCommand(envWithDefault("RELAY_AGENT_CODEX_COMMAND", "codex"))
	copilotImage := envWithDefault("RELAY_AGENT_COPILOT_IMAGE", containerImage)
	copilotCommand := envWithDefault("RELAY_AGENT_COPILOT_COMMAND", "cat")
	gitAuthorName := strings.TrimSpace(os.Getenv("RELAY_GIT_AUTHOR_NAME"))
	gitAuthorEmail := strings.TrimSpace(os.Getenv("RELAY_GIT_AUTHOR_EMAIL"))

	passthroughEnv := parseCSVList(envWithDefault(
		"RELAY_CONTAINER_PASSTHROUGH_ENV",
		"OPENAI_API_KEY,OPENAI_BASE_URL,OPENAI_ORG_ID,OPENAI_PROJECT",
	))
	containerEnv := collectProcessEnv(passthroughEnv)
	bridgeBatchIdle, err := envDurationMS("RELAY_BRIDGE_OUTPUT_BATCH_IDLE_MS", 300)
	if err != nil {
		return config{}, err
	}
	bridgeFlushMax, err := envDurationMSAllowZero("RELAY_BRIDGE_OUTPUT_FLUSH_MAX_MS", 2000)
	if err != nil {
		return config{}, err
	}
	bridgeDebugIO, err := envBool("RELAY_BRIDGE_DEBUG_IO", false)
	if err != nil {
		return config{}, err
	}
	archivePolicy, err := envRoomArchivePolicy("RELAY_SESSION_ROOM_ARCHIVE_POLICY", roomArchiveForget)
	if err != nil {
		return config{}, err
	}
	devImageTemplates, err := envBool("RELAY_DEV_IMAGE_TEMPLATES_ENABLED", false)
	if err != nil {
		return config{}, err
	}
	devImageBuildTO, err := envDurationSeconds("RELAY_DEV_IMAGE_BUILD_TIMEOUT_SEC", 600)
	if err != nil {
		return config{}, err
	}

	allowedUsers := parseCSVSet(os.Getenv("RELAY_ALLOWED_USERS"))

	return config{
		Matrix: matrixbot.Config{
			HomeserverURL:  homeserver,
			UserID:         userID,
			AccessToken:    accessToken,
			GovernorRoomID: governorRoomID,
		},
		WorkspaceBaseDir:    workspaceBaseDir,
		EventsDBPath:        eventsDBPath,
		EventsRetentionDays: eventsRetentionDays,
		ContainerRuntime:    containerRuntime,
		ContainerImage:      containerImage,
		CodexImage:          codexImage,
		CodexCommand:        codexCommand,
		CopilotImage:        copilotImage,
		CopilotCommand:      copilotCommand,
		ContainerEnv:        containerEnv,
		GitAuthorName:       gitAuthorName,
		GitAuthorEmail:      gitAuthorEmail,
		AllowedUsers:        allowedUsers,
		BridgeBatchIdle:     bridgeBatchIdle,
		BridgeFlushMax:      bridgeFlushMax,
		BridgeDebugIO:       bridgeDebugIO,
		RoomArchivePolicy:   archivePolicy,
		DevImageTemplates:   devImageTemplates,
		DevImageBuildTO:     devImageBuildTO,
	}, nil
}

func envDurationSeconds(key string, defaultSeconds int) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return time.Duration(defaultSeconds) * time.Second, nil
	}

	seconds, err := strconv.Atoi(value)
	if err != nil || seconds <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer (seconds)", key)
	}

	return time.Duration(seconds) * time.Second, nil
}

func envRoomArchivePolicy(key string, fallback roomArchivePolicy) (roomArchivePolicy, error) {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if value == "" {
		return fallback, nil
	}

	switch roomArchivePolicy(value) {
	case roomArchiveKeep, roomArchiveLeave, roomArchiveForget:
		return roomArchivePolicy(value), nil
	default:
		return "", fmt.Errorf("%s must be one of: keep, leave, forget", key)
	}
}

func envDurationMS(key string, defaultMS int) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return time.Duration(defaultMS) * time.Millisecond, nil
	}

	ms, err := strconv.Atoi(value)
	if err != nil || ms <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer (milliseconds)", key)
	}

	return time.Duration(ms) * time.Millisecond, nil
}

func envDurationMSAllowZero(key string, defaultMS int) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return time.Duration(defaultMS) * time.Millisecond, nil
	}

	ms, err := strconv.Atoi(value)
	if err != nil || ms < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer (milliseconds)", key)
	}

	return time.Duration(ms) * time.Millisecond, nil
}

func envNonNegativeInt(key string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", key)
	}

	return parsed, nil
}

func envBool(key string, fallback bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean (true/false)", key)
	}

	return parsed, nil
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
		return fmt.Sprintf("printenv OPENAI_API_KEY | codex login --with-api-key >/dev/null 2>&1 || true; %s", codexCommand)
	}
	return trimmed
}
