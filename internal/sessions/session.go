package sessions

import "time"

type State string

const (
	StateCreating           State = "creating"
	StatePreparingWorkspace State = "preparing_workspace"
	StateCreatingRoom       State = "creating_room"
	StateStartingContainer  State = "starting_container"
	StateRunning            State = "running"
	StateRestarting         State = "restarting"
	StateStopping           State = "stopping"
	StateExited             State = "exited"
	StateFailed             State = "failed"
)

type Session struct {
	ID             string
	Repo           string
	Branch         string
	Agent          string
	OwnerUserID    string
	GovernorRoomID string
	RoomID         string
	WorkspaceDir   string
	State          State
	CreatedAt      time.Time
}
