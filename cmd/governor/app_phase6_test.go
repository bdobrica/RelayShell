package main

import (
	"testing"

	"github.com/bdobrica/RelayShell/internal/sessions"
)

func TestShouldAutoRestoreSession(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		state sessions.State
		want  bool
	}{
		{name: "creating", state: sessions.StateCreating, want: true},
		{name: "preparing workspace", state: sessions.StatePreparingWorkspace, want: true},
		{name: "creating room", state: sessions.StateCreatingRoom, want: true},
		{name: "starting container", state: sessions.StateStartingContainer, want: true},
		{name: "running", state: sessions.StateRunning, want: true},
		{name: "committing", state: sessions.StateCommitting, want: true},
		{name: "restarting", state: sessions.StateRestarting, want: true},
		{name: "stopping", state: sessions.StateStopping, want: false},
		{name: "exited", state: sessions.StateExited, want: false},
		{name: "failed", state: sessions.StateFailed, want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := shouldAutoRestoreSession(tt.state); got != tt.want {
				t.Fatalf("shouldAutoRestoreSession(%q) = %t, want %t", tt.state, got, tt.want)
			}
		})
	}
}
