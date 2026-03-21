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
