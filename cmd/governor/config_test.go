package main

import (
	"strings"
	"testing"
)

func TestLoadConfig_DefaultRoomArchivePolicyIsForget(t *testing.T) {
	t.Setenv("RELAY_MATRIX_HOMESERVER", "http://localhost:8008")
	t.Setenv("RELAY_MATRIX_USER_ID", "@relayshell:localhost")
	t.Setenv("RELAY_MATRIX_ACCESS_TOKEN", "token")
	t.Setenv("RELAY_MATRIX_GOVERNOR_ROOM_ID", "!room:localhost")
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
	t.Setenv("RELAY_MATRIX_HOMESERVER", "http://localhost:8008")
	t.Setenv("RELAY_MATRIX_USER_ID", "@relayshell:localhost")
	t.Setenv("RELAY_MATRIX_ACCESS_TOKEN", "token")
	t.Setenv("RELAY_MATRIX_GOVERNOR_ROOM_ID", "!room:localhost")
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
	t.Setenv("RELAY_MATRIX_HOMESERVER", "http://localhost:8008")
	t.Setenv("RELAY_MATRIX_USER_ID", "@relayshell:localhost")
	t.Setenv("RELAY_MATRIX_ACCESS_TOKEN", "token")
	t.Setenv("RELAY_MATRIX_GOVERNOR_ROOM_ID", "!room:localhost")
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
	t.Setenv("RELAY_MATRIX_HOMESERVER", "http://localhost:8008")
	t.Setenv("RELAY_MATRIX_USER_ID", "@relayshell:localhost")
	t.Setenv("RELAY_MATRIX_ACCESS_TOKEN", "token")
	t.Setenv("RELAY_MATRIX_GOVERNOR_ROOM_ID", "!room:localhost")
	t.Setenv("RELAY_DEV_IMAGE_BUILD_TIMEOUT_SEC", "0")

	_, err := loadConfig()
	if err == nil {
		t.Fatal("loadConfig() expected error for invalid dev image build timeout")
	}

	if !strings.Contains(err.Error(), "RELAY_DEV_IMAGE_BUILD_TIMEOUT_SEC") {
		t.Fatalf("error = %q, expected env var name in message", err.Error())
	}
}
