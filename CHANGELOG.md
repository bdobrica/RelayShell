# Changelog

## 2026-03-21

Completed roadmap items were moved from `TODO.md` to keep the active TODO focused.

### Completed

- Phase 0: project/module structure initialized.
- Phase 0: Makefile, formatting, linting, and slog-based logging setup.
- Phase 1: Matrix bot connectivity, room join, sync loop ingestion, and message send path.
- Phase 1: Strict command parser with `/start`, `/restart`, `/exit`, `/commit`, `/status`, `/enter` parsing.
- Phase 1: Session model with room mapping in in-memory store.
- Phase 1: Per-session room creation and user invite flow.
- Phase 1: Workspace preparation using git clone + checkout.
- Phase 1: Container runner start/stop with stdin/stdout/stderr wiring.
- Phase 1: Message bridge forwarding Matrix input to agent stdin and streamed output back to Matrix.
- Phase 1: Session `/enter` command forwarding raw Enter.
- Phase 2: ANSI output rendering for terminal redraw-heavy output.
- Phase 2: Output idle batching and hard-cap flush behavior.
- Phase 2: Typing indicator while output buffering/flush waits.
- Phase 3: `/restart` command implementation (stop old process and start new process on same workspace).
- Phase 5: Codex backend image/command wiring.
- Phase 6: SQLite processed-event deduplication with automatic migrations and retention cleanup.

### Notes

- `/exit` is currently partial: it stops the bridge/process and removes session state, but workspace cleanup is still pending.
- `/commit` is parsed but still not implemented.
- Copilot backend remains a stub mapping and is not functionally implemented.