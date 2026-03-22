package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/bdobrica/RelayShell/internal/sessions"
)

func TestSessionPersistenceLifecycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "events.db")

	s, err := NewProcessedEventStore(ctx, dbPath)
	if err != nil {
		t.Fatalf("NewProcessedEventStore() error = %v", err)
	}
	defer func() {
		if closeErr := s.Close(); closeErr != nil {
			t.Fatalf("Close() error = %v", closeErr)
		}
	}()

	createdAt := time.Now().UTC().Round(0)
	session := &sessions.Session{
		ID:             "sess-1",
		Repo:           "https://github.com/example/repo.git",
		Branch:         "main",
		Agent:          "codex",
		OwnerUserID:    "@alice:localhost",
		GovernorRoomID: "!governor:localhost",
		RoomID:         "!session:localhost",
		WorkspaceDir:   "/tmp/workspace",
		DetectedStack:  "go",
		RuntimeImage:   "relayshell-codex:latest",
		State:          sessions.StateRunning,
		CreatedAt:      createdAt,
	}

	if err := s.UpsertSession(ctx, session); err != nil {
		t.Fatalf("UpsertSession() error = %v", err)
	}

	persisted, err := s.ListSessions(ctx)
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if len(persisted) != 1 {
		t.Fatalf("ListSessions() count = %d, want 1", len(persisted))
	}
	if persisted[0].ID != session.ID || persisted[0].State != sessions.StateRunning {
		t.Fatalf("unexpected persisted session: %+v", persisted[0])
	}
	if !persisted[0].CreatedAt.Equal(createdAt) {
		t.Fatalf("CreatedAt = %s, want %s", persisted[0].CreatedAt, createdAt)
	}

	session.State = sessions.StateFailed
	session.RuntimeImage = "relayshell-codex:v2"
	if err := s.UpsertSession(ctx, session); err != nil {
		t.Fatalf("UpsertSession() update error = %v", err)
	}

	persisted, err = s.ListSessions(ctx)
	if err != nil {
		t.Fatalf("ListSessions() after update error = %v", err)
	}
	if len(persisted) != 1 {
		t.Fatalf("ListSessions() after update count = %d, want 1", len(persisted))
	}
	if persisted[0].State != sessions.StateFailed {
		t.Fatalf("state after update = %q, want %q", persisted[0].State, sessions.StateFailed)
	}
	if persisted[0].RuntimeImage != "relayshell-codex:v2" {
		t.Fatalf("runtime image after update = %q", persisted[0].RuntimeImage)
	}

	if err := s.DeleteSession(ctx, session.ID); err != nil {
		t.Fatalf("DeleteSession() error = %v", err)
	}

	persisted, err = s.ListSessions(ctx)
	if err != nil {
		t.Fatalf("ListSessions() after delete error = %v", err)
	}
	if len(persisted) != 0 {
		t.Fatalf("ListSessions() after delete count = %d, want 0", len(persisted))
	}
}
