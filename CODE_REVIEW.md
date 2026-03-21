# RelayShell Code Review

Date: 2026-03-21
Scope: Current main branch implementation audited against TODO.md roadmap and runtime behavior.

## Post-Review Remediation Update

- Implemented workspace cleanup during session stop/exit.
   - Reference: `cmd/governor/app.go:377`.
- Implemented process-exit monitoring and room notification on unexpected agent exit.
   - References: `cmd/governor/app.go:313`, `cmd/governor/app.go:363`, `cmd/governor/app.go:389`.

These two items were high-severity at audit time and are now addressed in code.

## Findings (Ordered by Severity)

### High

1. Workspace cleanup is missing on session exit.
   - Evidence: `stopSession` stops bridge and removes session from in-memory store, but does not remove `session.WorkspaceDir`.
   - References: `cmd/governor/app.go:367`, `cmd/governor/app.go:375`.
   - Impact: Session workspaces can accumulate indefinitely and consume disk.

2. Container crash lifecycle is not monitored in app flow.
   - Evidence: `Process.Done()` exists, but no goroutine in app watches it after `bridge.Start`.
   - References: `internal/container/runner.go:153`, `cmd/governor/app.go:311`, `cmd/governor/app.go:360`.
   - Impact: Session may remain logically running after agent/container exits unexpectedly.

### Medium

1. PTY is not implemented yet; runtime still uses raw stdio pipes plus shell workaround.
   - Evidence: runner uses `StdinPipe`/`StdoutPipe`/`StderrPipe`; codex command is wrapped with `script` and `stty`.
   - References: `internal/container/runner.go:74`, `internal/container/runner.go:78`, `cmd/governor/config.go:190`.
   - Impact: Interactive agent behavior can still be fragile.

2. `/commit` command is parsed but not implemented.
   - Evidence: session handler returns a placeholder message.
   - Reference: `cmd/governor/app.go:223`.
   - Impact: Core lifecycle command remains incomplete.

3. Matrix sync retry uses fixed sleep without backoff strategy.
   - Evidence: sync loop sleeps 2 seconds on every error.
   - Reference: `cmd/governor/app.go:107`.
   - Impact: Less resilient behavior during prolonged homeserver outages.

### Low

1. Several Matrix send errors are intentionally ignored.
   - Evidence: many `_ = a.matrix.SendText(...)` calls.
   - References: `cmd/governor/app.go:139`, `cmd/governor/app.go:158`, `cmd/governor/app.go:325`.
   - Impact: Reduced observability for delivery failures.

2. Automated test suite is currently absent.
   - Evidence: `go test ./...` reports `[no test files]` for all packages.
   - Impact: Regression risk during refactoring.

## Implementation Status vs Previous TODO

Status legend: `Done`, `Partial`, `Not Done`

| Area | Item | Status | Evidence |
|---|---|---|---|
| Phase 0 | Repo/tooling setup | Done | module + Makefile + lint/fmt targets present |
| Phase 1 | Matrix bot connectivity and sync | Done | `internal/matrixbot/client.go` |
| Phase 1 | Strict command parsing | Done | `internal/sessions/command.go` |
| Phase 1 | Session model and in-memory mapping | Done | `internal/sessions/session.go`, `internal/store/session_store.go` |
| Phase 1 | Room creation/invite/metadata | Done | `cmd/governor/app.go:279`, `cmd/governor/app.go:317` |
| Phase 1 | Git clone + checkout workspace prep | Done | `internal/gitops/workspace.go:17` |
| Phase 1 | Container launch and stream bridge | Done | `internal/container/runner.go:43`, `internal/bridge/bridge.go:55` |
| Phase 1 | `/enter` command | Done | `cmd/governor/app.go:224` |
| Phase 2 | ANSI rendering and redraw handling | Done | `internal/bridge/ansi_renderer.go:230` |
| Phase 2 | Output buffering + flush hard cap | Done | `internal/bridge/bridge.go:108`, `internal/bridge/bridge.go:253` |
| Phase 2 | Matrix typing indicator while buffering | Done | `internal/bridge/bridge.go:236`, `internal/bridge/bridge.go:306` |
| Phase 2 | True PTY integration | Not Done | raw pipes still used |
| Phase 2 | Container crash handling | Not Done | app does not consume `Process.Done()` |
| Phase 2 | Broken pipe handling | Partial | EOF handled at bridge reader, no restart strategy |
| Phase 2 | Matrix reconnect strategy | Partial | fixed retry exists, no backoff/jitter |
| Phase 3 | `/restart` lifecycle | Done | `cmd/governor/app.go:330` |
| Phase 3 | `/exit` lifecycle | Partial | stop+remove from store done; workspace cleanup missing |
| Phase 3 | `/commit` lifecycle | Not Done | placeholder response |
| Phase 4 | Git worktrees/multi-session workspace optimization | Not Done | clone-per-session model |
| Phase 5 | Codex backend | Done | resolver + codex image/command wiring |
| Phase 5 | Copilot backend | Not Done | currently stubbed to `cat` behavior |
| Phase 6 | SQLite processed event dedup + migrations | Done | `internal/store/processed_event_store.go` |
| Phase 6 | Session persistence/restore | Not Done | session store is memory only |

## Test and Verification Snapshot

- `go test ./...` runs successfully, but all packages report no test files.
- No compilation errors observed during review.

## Recommended Next Priorities

1. Implement workspace cleanup on `/exit`.
2. Add container exit monitoring and room notification.
3. Implement `/commit` end-to-end.
4. Replace raw pipes with proper PTY handling.
5. Add unit tests for command parsing, ANSI renderer, and session lifecycle transitions.