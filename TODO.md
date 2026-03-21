# RelayShell TODO (Active)

Completed roadmap items were moved to `CHANGELOG.md` to keep this file focused on active work.

## Current Focus

### Phase 2 - PTY + Stability

- [x] Improve interactive CLI behavior under redraw-heavy workloads.

### Phase 3 - Session Lifecycle Commands

- [x] Complete `/exit` cleanup path:
  - [x] Decide and implement room archival strategy.

### Phase 4 - Git Improvements

- [x] Replace clone-per-session with worktree-based strategy.
- [x] Support multiple sessions per repository more efficiently.
- [x] Clean worktrees reliably on session stop/exit.

### Phase 4.5 - Template-Driven Dev Containers

- [ ] Build template-based worker image pipeline managed by governor.
- [ ] Detect repository stack (Go, Python, Node.js, mixed) during session startup.
- [ ] Add language templates with baseline dev tools:
  - [ ] Go template: Go, Python, make.
  - [ ] Python template: Python, build tools.
  - [ ] Node.js template: Node.js and common build tools.
- [ ] Python dependency bootstrap:
  - [ ] Discover and merge multiple requirements files.
  - [ ] Install merged dependency set in worker image/session startup.
- [ ] Define fallback behavior when stack detection is ambiguous.
- [ ] Add cache/rebuild policy for generated images.
- [ ] Add safeguards for install failures and timeout handling.

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
- [ ] Template selection precedence for mixed-language repositories.
- [ ] Requirements merge conflict policy (pin/version conflict handling).
- [ ] Session resumability guarantees.
- [ ] Command parsing strictness boundaries.
- [ ] Editable vs raw agent output policy.
