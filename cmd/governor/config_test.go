package main

import (
	"strings"
	"testing"
)

func seedRequiredMatrixEnv(t *testing.T) {
	t.Helper()
	t.Setenv("RELAY_MATRIX_HOMESERVER", "http://localhost:8008")
	t.Setenv("RELAY_MATRIX_USER_ID", "@relayshell:localhost")
	t.Setenv("RELAY_MATRIX_ACCESS_TOKEN", "token")
	t.Setenv("RELAY_MATRIX_GOVERNOR_ROOM_ID", "!room:localhost")
}

func TestLoadConfig_DefaultRoomArchivePolicyIsForget(t *testing.T) {
	seedRequiredMatrixEnv(t)
	t.Setenv("RELAY_SESSION_ROOM_ARCHIVE_POLICY", "")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}

	if cfg.RoomArchivePolicy != roomArchiveForget {
		t.Fatalf("RoomArchivePolicy = %q, want %q", cfg.RoomArchivePolicy, roomArchiveForget)
	}
}

func TestLoadConfig_InvalidRoomArchivePolicy(t *testing.T) {
	seedRequiredMatrixEnv(t)
	t.Setenv("RELAY_SESSION_ROOM_ARCHIVE_POLICY", "archive")

	_, err := loadConfig()
	if err == nil {
		t.Fatal("loadConfig() expected error for invalid archive policy")
	}

	if !strings.Contains(err.Error(), "RELAY_SESSION_ROOM_ARCHIVE_POLICY") {
		t.Fatalf("error = %q, expected env var name in message", err.Error())
	}
}

func TestLoadConfig_DefaultDevImageTemplateSettings(t *testing.T) {
	seedRequiredMatrixEnv(t)
	t.Setenv("RELAY_DEV_IMAGE_TEMPLATES_ENABLED", "")
	t.Setenv("RELAY_DEV_IMAGE_BUILD_TIMEOUT_SEC", "")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}

	if cfg.DevImageTemplates {
		t.Fatal("DevImageTemplates should default to false")
	}
	if cfg.DevImageBuildTO <= 0 {
		t.Fatalf("DevImageBuildTO should be > 0, got %s", cfg.DevImageBuildTO)
	}
}

func TestLoadConfig_InvalidDevImageBuildTimeout(t *testing.T) {
	seedRequiredMatrixEnv(t)
	t.Setenv("RELAY_DEV_IMAGE_BUILD_TIMEOUT_SEC", "0")

	_, err := loadConfig()
	if err == nil {
		t.Fatal("loadConfig() expected error for invalid dev image build timeout")
	}

	if !strings.Contains(err.Error(), "RELAY_DEV_IMAGE_BUILD_TIMEOUT_SEC") {
		t.Fatalf("error = %q, expected env var name in message", err.Error())
	}
}

func TestLoadConfig_DefaultCopilotBackend(t *testing.T) {
	seedRequiredMatrixEnv(t)
	t.Setenv("RELAY_AGENT_COPILOT_IMAGE", "")
	t.Setenv("RELAY_AGENT_COPILOT_COMMAND", "")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}

	if cfg.CopilotImage != "relayshell-copilot:latest" {
		t.Fatalf("CopilotImage = %q, want relayshell-copilot:latest", cfg.CopilotImage)
	}
	if !strings.Contains(cfg.CopilotCommand, "copilot") {
		t.Fatalf("CopilotCommand = %q, expected copilot command", cfg.CopilotCommand)
	}
	if !strings.Contains(cfg.CopilotCommand, "GH_TOKEN") {
		t.Fatalf("CopilotCommand = %q, expected GH_TOKEN bootstrap", cfg.CopilotCommand)
	}
}

func TestLoadConfig_Phase7ContainerSecuritySettings(t *testing.T) {
	seedRequiredMatrixEnv(t)
	t.Setenv("RELAY_CONTAINER_RUN_AS_NON_ROOT", "true")
	t.Setenv("RELAY_CONTAINER_RUN_AS_USER", "1001:1001")
	t.Setenv("RELAY_CONTAINER_CPU_LIMIT", "1.5")
	t.Setenv("RELAY_CONTAINER_MEMORY_LIMIT", "2g")
	t.Setenv("RELAY_CONTAINER_NETWORK", "none")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}

	if cfg.ContainerRunAsUser != "1001:1001" {
		t.Fatalf("ContainerRunAsUser = %q, want 1001:1001", cfg.ContainerRunAsUser)
	}
	if cfg.ContainerCPULimit != "1.5" {
		t.Fatalf("ContainerCPULimit = %q, want 1.5", cfg.ContainerCPULimit)
	}
	if cfg.ContainerMemory != "2g" {
		t.Fatalf("ContainerMemory = %q, want 2g", cfg.ContainerMemory)
	}
	if cfg.ContainerNetwork != "none" {
		t.Fatalf("ContainerNetwork = %q, want none", cfg.ContainerNetwork)
	}
}

func TestLoadConfig_InvalidContainerCPULimit(t *testing.T) {
	seedRequiredMatrixEnv(t)
	t.Setenv("RELAY_CONTAINER_CPU_LIMIT", "0")

	_, err := loadConfig()
	if err == nil {
		t.Fatal("loadConfig() expected error for invalid cpu limit")
	}

	if !strings.Contains(err.Error(), "RELAY_CONTAINER_CPU_LIMIT") {
		t.Fatalf("error = %q, expected env var name in message", err.Error())
	}
}
