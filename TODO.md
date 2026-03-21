# RelayShell TODO (Active)

Completed roadmap items were moved to `CHANGELOG.md` to keep this file focused on active work.

## Current Focus

### Phase 2 - PTY + Stability

- [ ] Replace raw stdio pipes with true PTY integration.
- [ ] Improve interactive CLI behavior under redraw-heavy workloads.
- [ ] Add robust container crash detection and session-state transitions on unexpected exits.
- [ ] Improve broken-pipe/error recovery strategy (beyond current EOF handling).
- [ ] Add Matrix reconnect backoff and recovery strategy.

### Phase 3 - Session Lifecycle Commands

- [ ] Implement `/commit` end-to-end:
  - [ ] Generate git diff summary.
  - [ ] Generate commit message (agent-assisted or fallback).
  - [ ] Run `git add -A` and `git commit`.
  - [ ] Return commit SHA and summary to session room.
- [ ] Complete `/exit` cleanup path:
  - [ ] Delete workspace directory on exit.
  - [ ] Decide and implement room archival strategy.

### Phase 4 - Git Improvements

- [ ] Replace clone-per-session with worktree-based strategy.
- [ ] Support multiple sessions per repository more efficiently.
- [ ] Clean worktrees reliably on session stop/exit.

### Phase 5 - Agent Backends

- [ ] Implement real Copilot backend:
  - [ ] Container image.
  - [ ] CLI setup.
  - [ ] Auth handling.
- [ ] Define and adopt an explicit AgentBackend interface for all backends.

### Phase 6 - Persistence

- [ ] Persist live session state (not only processed Matrix events).
- [ ] Restore sessions after governor restart.

### Phase 7 - Security & Isolation

- [ ] Run worker containers as non-root.
- [ ] Apply CPU/memory limits.
- [ ] Optional network restrictions per worker.
- [ ] Strengthen secret handling practices.

### Phase 8 - UX Improvements

- [ ] Improve formatting and readability of streamed output.
- [ ] Code-block detection for agent responses.
- [ ] Better status and lifecycle notifications.
- [ ] Session summaries.

### Phase 9 - Advanced Features

- [ ] Multi-user support.
- [ ] Fine-grained permissions.
- [ ] Session timeouts.
- [ ] Session resume support.
- [ ] Logs/history export.

### Phase 10 - Observability

- [ ] Structured logging improvements.
- [ ] Metrics (session counts, errors, throughput).
- [ ] Debug mode hardening.

## Nice-to-Have

- [ ] Web dashboard.
- [ ] GitHub PR integration.
- [ ] Auto-push after commit.
- [ ] Diff preview before commit.

## Open Questions

- [ ] Copilot auth strategy in containers.
- [ ] Session resumability guarantees.
- [ ] Command parsing strictness boundaries.
- [ ] Editable vs raw agent output policy.
