package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bdobrica/RelayShell/internal/agents"
	"github.com/bdobrica/RelayShell/internal/bridge"
	"github.com/bdobrica/RelayShell/internal/container"
	"github.com/bdobrica/RelayShell/internal/devimage"
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
	events   *store.ProcessedEventStore

	bridgeMu sync.RWMutex
	bridges  map[string]*bridge.Bridge
}

func newApp(cfg config, logger *slog.Logger) (*app, error) {
	matrixClient, err := matrixbot.NewClient(cfg.Matrix, logger.With("component", "matrixbot"))
	if err != nil {
		return nil, fmt.Errorf("init matrix client: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(cfg.EventsDBPath), 0o755); err != nil {
		return nil, fmt.Errorf("create events db directory: %w", err)
	}

	eventsStore, err := store.NewProcessedEventStore(context.Background(), cfg.EventsDBPath)
	if err != nil {
		return nil, fmt.Errorf("init processed event store: %w", err)
	}

	return &app{
		cfg:    cfg,
		logger: logger,
		matrix: matrixClient,
		git:    gitops.NewManager(cfg.WorkspaceBaseDir, cfg.GitAuthorName, cfg.GitAuthorEmail),
		runner: container.NewRunner(cfg.ContainerRuntime, logger.With("component", "container")),
		agents: agents.Resolver{
			DefaultImage:   cfg.ContainerImage,
			CodexImage:     cfg.CodexImage,
			CodexCommand:   cfg.CodexCommand,
			CopilotImage:   cfg.CopilotImage,
			CopilotCommand: cfg.CopilotCommand,
		},
		sessions: store.NewSessionStore(),
		events:   eventsStore,
		bridges:  map[string]*bridge.Bridge{},
	}, nil
}

func (a *app) run(ctx context.Context) error {
	defer func() {
		if err := a.events.Close(); err != nil {
			a.logger.Error("close processed event store", "error", err)
		}
	}()

	if err := a.matrix.JoinRoom(ctx, a.cfg.Matrix.GovernorRoomID); err != nil {
		return fmt.Errorf("join governor room: %w", err)
	}

	if a.cfg.EventsRetentionDays > 0 {
		cutoff := time.Now().UTC().AddDate(0, 0, -a.cfg.EventsRetentionDays)
		deleted, err := a.events.DeleteProcessedBefore(ctx, cutoff)
		if err != nil {
			a.logger.Error("cleanup processed events failed", "error", err, "retention_days", a.cfg.EventsRetentionDays)
		} else if deleted > 0 {
			a.logger.Info("cleaned processed events", "deleted_rows", deleted, "retention_days", a.cfg.EventsRetentionDays)
		}
	}

	a.logger.Info("connected to governor room", "room_id", a.cfg.Matrix.GovernorRoomID)

	since := ""
	retryDelay := time.Second
	const maxRetryDelay = 30 * time.Second
	for {
		select {
		case <-ctx.Done():
			a.shutdownActiveSessions()
			return nil
		default:
		}

		nextBatch, events, err := a.matrix.SyncOnce(ctx, since, 30*time.Second)
		if err != nil {
			a.logger.Error("matrix sync failed", "error", err, "retry_in", retryDelay)
			select {
			case <-ctx.Done():
				a.shutdownActiveSessions()
				return nil
			case <-time.After(retryDelay):
			}

			retryDelay *= 2
			if retryDelay > maxRetryDelay {
				retryDelay = maxRetryDelay
			}
			continue
		}

		if retryDelay != time.Second {
			a.logger.Info("matrix sync recovered", "retry_delay_reset", true)
			retryDelay = time.Second
		}
		since = nextBatch

		for _, event := range events {
			if event.Sender == a.matrix.UserID() {
				continue
			}

			alreadyProcessed, err := a.events.IsProcessed(ctx, event.EventID)
			if err != nil {
				a.logger.Error("check processed event failed", "event_id", event.EventID, "error", err)
				continue
			}
			if alreadyProcessed {
				continue
			}

			a.handleEvent(ctx, event)
			if err := a.events.MarkProcessed(ctx, event.EventID, event.RoomID); err != nil {
				a.logger.Error("mark processed event failed", "event_id", event.EventID, "room_id", event.RoomID, "error", err)
			}
		}
	}
}

func (a *app) shutdownActiveSessions() {
	activeSessions := a.sessions.List()
	if len(activeSessions) == 0 {
		return
	}

	a.logger.Info("shutting down active sessions", "count", len(activeSessions))

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, session := range activeSessions {
		a.stopSession(shutdownCtx, session)
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
		_ = a.matrix.SendText(ctx, event.RoomID, "Governor room commands: /start repo=<repo> branch=<branch> agent=<agent> and /status")
		return
	}

	switch cmd.Name {
	case sessions.CommandStatus:
		activeSessions := a.sessions.List()
		if len(activeSessions) == 0 {
			_ = a.matrix.SendText(ctx, event.RoomID, "No active sessions")
			return
		}

		lines := []string{"Active sessions:"}
		for _, session := range activeSessions {
			lines = append(lines, fmt.Sprintf("- %s | %s | room=%s | repo=%s | branch=%s", session.ID, session.State, session.RoomID, session.Repo, session.Branch))
		}
		_ = a.matrix.SendText(ctx, event.RoomID, strings.Join(lines, "\n"))
		return
	case sessions.CommandStart:
		// handled below
	default:
		_ = a.matrix.SendText(ctx, event.RoomID, "Governor room supports /start and /status")
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
			if errors.Is(err, sessions.ErrUnsupportedCommand) {
				bridgeRef, ok := a.getBridge(session.RoomID)
				if !ok {
					_ = a.matrix.SendText(ctx, event.RoomID, "No active container bridge for this session")
					return
				}
				if err := bridgeRef.ForwardInput(text); err != nil {
					a.handleBridgeInputError(ctx, session, event.RoomID, err)
				}
				return
			}

			_ = a.matrix.SendText(ctx, event.RoomID, "Invalid command: "+err.Error())
			return
		}

		switch cmd.Name {
		case sessions.CommandStatus:
			_ = a.matrix.SendText(ctx, event.RoomID, fmt.Sprintf("Session %s is %s", session.ID, session.State))
		case sessions.CommandCommit:
			a.commitSession(ctx, session)
		case sessions.CommandEnter:
			bridgeRef, ok := a.getBridge(session.RoomID)
			if !ok {
				_ = a.matrix.SendText(ctx, event.RoomID, "No active container bridge for this session")
				return
			}
			if err := bridgeRef.ForwardInput(""); err != nil {
				a.handleBridgeInputError(ctx, session, event.RoomID, err)
			}
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
		a.handleBridgeInputError(ctx, session, event.RoomID, err)
	}
}

func (a *app) handleBridgeInputError(ctx context.Context, session *sessions.Session, roomID string, err error) {
	a.logger.Error("forward message to container failed", "session_id", session.ID, "error", err)

	if errors.Is(err, container.ErrBrokenPipe) || errors.Is(err, container.ErrProcessExited) {
		session.State = sessions.StateFailed
		if bridgeRef, ok := a.getBridge(session.RoomID); ok {
			_ = bridgeRef.Stop()
		}
		a.deleteBridge(session.RoomID)
		_ = a.matrix.SendText(ctx, roomID, "Agent input channel is unavailable (process exited or pipe closed). Use /restart to recover.")
		return
	}

	_ = a.matrix.SendText(ctx, roomID, "Failed to send message to agent")
}

func (a *app) commitSession(ctx context.Context, session *sessions.Session) {
	if strings.TrimSpace(session.WorkspaceDir) == "" {
		_ = a.matrix.SendText(ctx, session.RoomID, "No workspace available for commit")
		return
	}

	previousState := session.State
	session.State = sessions.StateCommitting

	result, err := a.git.CommitAll(ctx, session.WorkspaceDir)
	if err != nil {
		session.State = previousState
		if errors.Is(err, gitops.ErrNoChanges) {
			_ = a.matrix.SendText(ctx, session.RoomID, "No changes to commit")
			return
		}

		a.logger.Error("commit session failed", "session_id", session.ID, "workspace", session.WorkspaceDir, "error", err)
		_ = a.matrix.SendText(ctx, session.RoomID, "Commit failed: "+err.Error())
		return
	}

	session.State = previousState

	summary := []string{
		"Commit created",
		"sha=" + result.SHA,
		"message=" + result.Message,
		fmt.Sprintf("files=%d", len(result.Files)),
	}
	if len(result.Files) > 0 {
		limit := len(result.Files)
		if limit > 8 {
			limit = 8
		}
		summary = append(summary, "changed="+strings.Join(result.Files[:limit], ", "))
		if len(result.Files) > limit {
			summary = append(summary, fmt.Sprintf("and %d more file(s)", len(result.Files)-limit))
		}
	}

	_ = a.matrix.SendText(ctx, session.RoomID, strings.Join(summary, "\n"))
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

	if a.cfg.DevImageTemplates {
		_ = a.matrix.SendText(ctx, session.GovernorRoomID, fmt.Sprintf("Building %s container...", session.Agent))
	}

	runtimeImage, detectedStack, buildAttempted, buildErr := a.resolveRuntimeImage(ctx, session, agentSpec.Image)
	if a.cfg.DevImageTemplates {
		switch {
		case buildAttempted && buildErr == nil:
			_ = a.matrix.SendText(ctx, session.GovernorRoomID, fmt.Sprintf("%s container built.", session.Agent))
		case buildAttempted && buildErr != nil:
			_ = a.matrix.SendText(ctx, session.GovernorRoomID, fmt.Sprintf("%s container build failed, using base image.", session.Agent))
		default:
			_ = a.matrix.SendText(ctx, session.GovernorRoomID, fmt.Sprintf("%s container ready (no custom build needed).", session.Agent))
		}
	}

	session.RuntimeImage = runtimeImage
	session.DetectedStack = string(detectedStack)

	proc, err := a.runner.Start(ctx, container.StartOptions{
		SessionID:    session.ID,
		WorkspaceDir: session.WorkspaceDir,
		Image:        runtimeImage,
		Command:      agentSpec.Command,
		Env:          a.cfg.ContainerEnv,
	})
	if err != nil {
		session.State = sessions.StateFailed
		return nil, err
	}

	bridgeRef := bridge.New(a.logger.With("session_id", session.ID), a.matrix, session.RoomID, proc, a.cfg.BridgeBatchIdle, a.cfg.BridgeFlushMax, a.cfg.BridgeDebugIO)
	bridgeRef.Start(ctx)
	a.setBridge(session.RoomID, bridgeRef)
	a.watchProcessExit(session, proc)

	session.State = sessions.StateRunning
	a.sessions.Add(session)

	metadata := strings.Join([]string{
		"RelayShell session started",
		"id=" + session.ID,
		"repo=" + session.Repo,
		"branch=" + session.Branch,
		"agent=" + session.Agent,
		"stack=" + session.DetectedStack,
		"image=" + session.RuntimeImage,
		"workspace=" + session.WorkspaceDir,
	}, "\n")
	_ = a.matrix.SendText(ctx, session.RoomID, metadata)

	return session, nil
}

func (a *app) resolveRuntimeImage(ctx context.Context, session *sessions.Session, baseImage string) (string, devimage.Stack, bool, error) {
	detectedStack, err := devimage.DetectStack(session.WorkspaceDir)
	if err != nil {
		a.logger.Warn("stack detection failed; using base image", "session_id", session.ID, "workspace", session.WorkspaceDir, "error", err)
		return baseImage, devimage.StackUnknown, false, err
	}

	if !a.cfg.DevImageTemplates {
		return baseImage, detectedStack, false, nil
	}

	if detectedStack == devimage.StackUnknown {
		return baseImage, detectedStack, false, nil
	}

	derivedImage, err := devimage.BuildDerivedImage(
		ctx,
		a.cfg.ContainerRuntime,
		session.WorkspaceDir,
		session.ID,
		detectedStack,
		a.cfg.DevImageBuildTO,
	)
	if err != nil {
		a.logger.Warn("derived dev image build failed; falling back to base image", "session_id", session.ID, "stack", detectedStack, "error", err)
		return baseImage, detectedStack, true, err
	}

	a.logger.Info("derived dev image built", "session_id", session.ID, "stack", detectedStack, "image", derivedImage)
	return derivedImage, detectedStack, true, nil
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

	runtimeImage := session.RuntimeImage
	if strings.TrimSpace(runtimeImage) == "" {
		runtimeImage, detectedStack, _, _ := a.resolveRuntimeImage(ctx, session, agentSpec.Image)
		session.RuntimeImage = runtimeImage
		session.DetectedStack = string(detectedStack)
	}

	proc, err := a.runner.Start(ctx, container.StartOptions{
		SessionID:    session.ID,
		WorkspaceDir: session.WorkspaceDir,
		Image:        runtimeImage,
		Command:      agentSpec.Command,
		Env:          a.cfg.ContainerEnv,
	})
	if err != nil {
		session.State = sessions.StateFailed
		a.logger.Error("restart session failed", "session_id", session.ID, "error", err)
		_ = a.matrix.SendText(ctx, session.RoomID, "Failed to restart session")
		return
	}

	bridgeRef := bridge.New(a.logger.With("session_id", session.ID), a.matrix, session.RoomID, proc, a.cfg.BridgeBatchIdle, a.cfg.BridgeFlushMax, a.cfg.BridgeDebugIO)
	bridgeRef.Start(ctx)
	a.setBridge(session.RoomID, bridgeRef)
	a.watchProcessExit(session, proc)

	session.State = sessions.StateRunning
	_ = a.matrix.SendText(ctx, session.RoomID, "Session restarted")
}

func (a *app) stopSession(ctx context.Context, session *sessions.Session) {
	session.State = sessions.StateStopping

	if bridgeRef, ok := a.getBridge(session.RoomID); ok {
		_ = bridgeRef.Stop()
		a.deleteBridge(session.RoomID)
	}

	if session.WorkspaceDir != "" {
		if err := a.git.CleanupWorkspace(ctx, session.WorkspaceDir); err != nil {
			a.logger.Error("remove workspace failed", "session_id", session.ID, "workspace", session.WorkspaceDir, "error", err)
			_ = a.matrix.SendText(ctx, session.RoomID, "Session stopped, but workspace cleanup failed: "+err.Error())
		}
	}

	a.sessions.Delete(session.ID)
	session.State = sessions.StateExited
	_ = a.matrix.SendText(ctx, session.RoomID, "Session stopped")

	a.applyRoomArchivePolicy(session)
}

func (a *app) applyRoomArchivePolicy(session *sessions.Session) {
	if strings.TrimSpace(session.RoomID) == "" {
		return
	}

	archiveCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	switch a.cfg.RoomArchivePolicy {
	case roomArchiveKeep:
		return
	case roomArchiveLeave:
		if err := a.matrix.LeaveRoom(archiveCtx, session.RoomID); err != nil {
			a.logger.Error("leave session room failed", "session_id", session.ID, "room_id", session.RoomID, "error", err)
		}
	case roomArchiveForget:
		if err := a.matrix.LeaveRoom(archiveCtx, session.RoomID); err != nil {
			a.logger.Error("leave session room before forget failed", "session_id", session.ID, "room_id", session.RoomID, "error", err)
			return
		}
		if err := a.matrix.ForgetRoom(archiveCtx, session.RoomID); err != nil {
			a.logger.Error("forget session room failed", "session_id", session.ID, "room_id", session.RoomID, "error", err)
		}
	default:
		a.logger.Error("unknown room archive policy", "policy", a.cfg.RoomArchivePolicy)
	}
}

func (a *app) watchProcessExit(session *sessions.Session, proc *container.Process) {
	go func() {
		err, ok := <-proc.Done()
		if !ok {
			return
		}

		if session.State == sessions.StateStopping || session.State == sessions.StateRestarting || session.State == sessions.StateExited {
			return
		}

		if errors.Is(err, context.Canceled) {
			return
		}

		a.deleteBridge(session.RoomID)

		message := "Agent process exited. Use /restart to start it again."
		if err != nil {
			session.State = sessions.StateFailed
			message = "Agent process exited unexpectedly: " + err.Error()
		} else {
			session.State = sessions.StateExited
		}

		if sendErr := a.matrix.SendText(context.Background(), session.RoomID, message); sendErr != nil {
			a.logger.Error("send process exit notification failed", "session_id", session.ID, "room_id", session.RoomID, "error", sendErr)
		}
	}()
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
