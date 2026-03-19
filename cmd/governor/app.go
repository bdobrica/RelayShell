package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/bdobrica/RelayShell/internal/agents"
	"github.com/bdobrica/RelayShell/internal/bridge"
	"github.com/bdobrica/RelayShell/internal/container"
	"github.com/bdobrica/RelayShell/internal/gitops"
	"github.com/bdobrica/RelayShell/internal/matrixbot"
	"github.com/bdobrica/RelayShell/internal/sessions"
	"github.com/bdobrica/RelayShell/internal/store"
)

type app struct {
	cfg config

	logger   *slog.Logger
	matrix   *matrixbot.Client
	git      *gitops.Manager
	runner   *container.Runner
	agents   agents.Resolver
	sessions *store.SessionStore

	bridgeMu sync.RWMutex
	bridges  map[string]*bridge.Bridge
}

func newApp(cfg config, logger *slog.Logger) (*app, error) {
	matrixClient, err := matrixbot.NewClient(cfg.Matrix, logger.With("component", "matrixbot"))
	if err != nil {
		return nil, fmt.Errorf("init matrix client: %w", err)
	}

	return &app{
		cfg:    cfg,
		logger: logger,
		matrix: matrixClient,
		git:    gitops.NewManager(cfg.WorkspaceBaseDir),
		runner: container.NewRunner(cfg.ContainerRuntime, logger.With("component", "container")),
		agents: agents.Resolver{
			DefaultImage:   cfg.ContainerImage,
			CodexImage:     cfg.CodexImage,
			CodexCommand:   cfg.CodexCommand,
			CopilotImage:   cfg.CopilotImage,
			CopilotCommand: cfg.CopilotCommand,
		},
		sessions: store.NewSessionStore(),
		bridges:  map[string]*bridge.Bridge{},
	}, nil
}

func (a *app) run(ctx context.Context) error {
	if err := a.matrix.JoinRoom(ctx, a.cfg.Matrix.GovernorRoomID); err != nil {
		return fmt.Errorf("join governor room: %w", err)
	}

	a.logger.Info("connected to governor room", "room_id", a.cfg.Matrix.GovernorRoomID)

	since := ""
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		nextBatch, events, err := a.matrix.SyncOnce(ctx, since, 30*time.Second)
		if err != nil {
			a.logger.Error("matrix sync failed", "error", err)
			time.Sleep(2 * time.Second)
			continue
		}
		since = nextBatch

		for _, event := range events {
			if event.Sender == a.matrix.UserID() {
				continue
			}
			a.handleEvent(ctx, event)
		}
	}
}

func (a *app) handleEvent(ctx context.Context, event matrixbot.Event) {
	if event.MsgType != "m.text" {
		return
	}
	if !a.isAllowedUser(event.Sender) {
		_ = a.matrix.SendText(ctx, event.RoomID, "User is not allowed to control RelayShell")
		return
	}

	if event.RoomID == a.cfg.Matrix.GovernorRoomID {
		a.handleGovernorEvent(ctx, event)
		return
	}

	session, ok := a.sessions.GetByRoomID(event.RoomID)
	if !ok {
		return
	}
	a.handleSessionEvent(ctx, session, event)
}

func (a *app) handleGovernorEvent(ctx context.Context, event matrixbot.Event) {
	cmd, err := sessions.ParseCommand(event.Body)
	if err != nil {
		_ = a.matrix.SendText(ctx, event.RoomID, "Governor room accepts only valid commands. Example: /start repo=<repo> branch=<branch> agent=<agent>")
		return
	}

	if cmd.Name != sessions.CommandStart {
		_ = a.matrix.SendText(ctx, event.RoomID, "Governor room only accepts /start commands")
		return
	}

	session, err := a.startSession(ctx, event.Sender, cmd)
	if err != nil {
		a.logger.Error("create session failed", "error", err)
		_ = a.matrix.SendText(ctx, event.RoomID, "Session creation failed: "+err.Error())
		return
	}

	_ = a.matrix.SendText(ctx, event.RoomID, fmt.Sprintf("Session %s created in room %s", session.ID, session.RoomID))
}

func (a *app) handleSessionEvent(ctx context.Context, session *sessions.Session, event matrixbot.Event) {
	text := strings.TrimSpace(event.Body)
	if text == "" {
		return
	}

	if strings.HasPrefix(text, "/") {
		cmd, err := sessions.ParseCommand(text)
		if err != nil {
			_ = a.matrix.SendText(ctx, event.RoomID, "Invalid command: "+err.Error())
			return
		}

		switch cmd.Name {
		case sessions.CommandStatus:
			_ = a.matrix.SendText(ctx, event.RoomID, fmt.Sprintf("Session %s is %s", session.ID, session.State))
		case sessions.CommandCommit:
			_ = a.matrix.SendText(ctx, event.RoomID, "/commit is parsed but implemented in Phase 3")
		case sessions.CommandRestart:
			a.restartSession(ctx, session)
		case sessions.CommandExit:
			a.stopSession(ctx, session)
		default:
			_ = a.matrix.SendText(ctx, event.RoomID, "Unsupported command in session room")
		}
		return
	}

	bridgeRef, ok := a.getBridge(session.RoomID)
	if !ok {
		_ = a.matrix.SendText(ctx, event.RoomID, "No active container bridge for this session")
		return
	}

	if err := bridgeRef.ForwardInput(text); err != nil {
		a.logger.Error("forward message to container failed", "error", err)
		_ = a.matrix.SendText(ctx, event.RoomID, "Failed to send message to agent")
	}
}

func (a *app) startSession(ctx context.Context, ownerUserID string, cmd sessions.Command) (*sessions.Session, error) {
	sessionID := generateSessionID()

	session := &sessions.Session{
		ID:             sessionID,
		Repo:           cmd.Repo,
		Branch:         cmd.Branch,
		Agent:          cmd.Agent,
		OwnerUserID:    ownerUserID,
		GovernorRoomID: a.cfg.Matrix.GovernorRoomID,
		State:          sessions.StateCreating,
		CreatedAt:      time.Now().UTC(),
	}

	session.State = sessions.StatePreparingWorkspace
	workspaceDir, err := a.git.Prepare(ctx, session.ID, session.Repo, session.Branch)
	if err != nil {
		session.State = sessions.StateFailed
		return nil, err
	}
	session.WorkspaceDir = workspaceDir

	session.State = sessions.StateCreatingRoom
	roomID, err := a.matrix.CreateRoom(
		ctx,
		fmt.Sprintf("RelayShell Session %s", session.ID),
		fmt.Sprintf("repo=%s branch=%s agent=%s", session.Repo, session.Branch, session.Agent),
		[]string{ownerUserID},
	)
	if err != nil {
		session.State = sessions.StateFailed
		return nil, err
	}
	session.RoomID = roomID

	session.State = sessions.StateStartingContainer
	agentSpec, err := a.agents.Resolve(session.Agent)
	if err != nil {
		session.State = sessions.StateFailed
		return nil, err
	}

	proc, err := a.runner.Start(ctx, container.StartOptions{
		SessionID:    session.ID,
		WorkspaceDir: session.WorkspaceDir,
		Image:        agentSpec.Image,
		Command:      agentSpec.Command,
		Env:          a.cfg.ContainerEnv,
	})
	if err != nil {
		session.State = sessions.StateFailed
		return nil, err
	}

	bridgeRef := bridge.New(a.logger.With("session_id", session.ID), a.matrix, session.RoomID, proc)
	bridgeRef.Start(ctx)
	a.setBridge(session.RoomID, bridgeRef)

	session.State = sessions.StateRunning
	a.sessions.Add(session)

	metadata := strings.Join([]string{
		"RelayShell session started",
		"id=" + session.ID,
		"repo=" + session.Repo,
		"branch=" + session.Branch,
		"agent=" + session.Agent,
		"workspace=" + session.WorkspaceDir,
	}, "\n")
	_ = a.matrix.SendText(ctx, session.RoomID, metadata)

	return session, nil
}

func (a *app) restartSession(ctx context.Context, session *sessions.Session) {
	session.State = sessions.StateRestarting

	if oldBridge, ok := a.getBridge(session.RoomID); ok {
		_ = oldBridge.Stop()
	}

	agentSpec, err := a.agents.Resolve(session.Agent)
	if err != nil {
		session.State = sessions.StateFailed
		a.logger.Error("agent resolution failed", "session_id", session.ID, "error", err)
		_ = a.matrix.SendText(ctx, session.RoomID, "Failed to resolve agent runtime")
		return
	}

	proc, err := a.runner.Start(ctx, container.StartOptions{
		SessionID:    session.ID,
		WorkspaceDir: session.WorkspaceDir,
		Image:        agentSpec.Image,
		Command:      agentSpec.Command,
		Env:          a.cfg.ContainerEnv,
	})
	if err != nil {
		session.State = sessions.StateFailed
		a.logger.Error("restart session failed", "session_id", session.ID, "error", err)
		_ = a.matrix.SendText(ctx, session.RoomID, "Failed to restart session")
		return
	}

	bridgeRef := bridge.New(a.logger.With("session_id", session.ID), a.matrix, session.RoomID, proc)
	bridgeRef.Start(ctx)
	a.setBridge(session.RoomID, bridgeRef)

	session.State = sessions.StateRunning
	_ = a.matrix.SendText(ctx, session.RoomID, "Session restarted")
}

func (a *app) stopSession(ctx context.Context, session *sessions.Session) {
	session.State = sessions.StateStopping

	if bridgeRef, ok := a.getBridge(session.RoomID); ok {
		_ = bridgeRef.Stop()
		a.deleteBridge(session.RoomID)
	}

	a.sessions.Delete(session.ID)
	session.State = sessions.StateExited
	_ = a.matrix.SendText(ctx, session.RoomID, "Session stopped")
}

func (a *app) isAllowedUser(userID string) bool {
	if len(a.cfg.AllowedUsers) == 0 {
		return true
	}
	_, ok := a.cfg.AllowedUsers[userID]
	return ok
}

func (a *app) getBridge(roomID string) (*bridge.Bridge, bool) {
	a.bridgeMu.RLock()
	defer a.bridgeMu.RUnlock()
	br, ok := a.bridges[roomID]
	return br, ok
}

func (a *app) setBridge(roomID string, bridgeRef *bridge.Bridge) {
	a.bridgeMu.Lock()
	defer a.bridgeMu.Unlock()
	a.bridges[roomID] = bridgeRef
}

func (a *app) deleteBridge(roomID string) {
	a.bridgeMu.Lock()
	defer a.bridgeMu.Unlock()
	delete(a.bridges, roomID)
}

func generateSessionID() string {
	random := make([]byte, 4)
	if _, err := rand.Read(random); err != nil {
		return fmt.Sprintf("sess-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("sess-%d-%x", time.Now().Unix(), random)
}
