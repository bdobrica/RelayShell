# Changelog

## 2026-03-21

Completed roadmap items were moved from `TODO.md` to keep the active TODO focused.

### Follow-up Fixes

- Added session workspace cleanup on `/exit` session stop path.
- Added process exit watcher to detect unexpected container exits, transition session state, and notify the session room.
- Added PTY-first container startup (`docker run -it`) with fallback to pipe mode when PTY allocation fails.
- Updated Codex command normalization for PTY runtime (direct `codex --no-alt-screen` after login step).
- Added exponential Matrix sync retry backoff (capped) with delay reset after successful recovery.
- Added explicit broken-pipe/process-exit input recovery with deterministic session failure state and restart guidance.
- Implemented `/commit`: stage all changes, generate fallback commit message, create commit, and report short SHA + changed files.
- Added `/commit` author identity resolution precedence: `RELAY_GIT_AUTHOR_*` env vars, then host global git config, then RelayShell defaults.
- Improved redraw-heavy interactive output handling by preserving ANSI screen state across bridge flushes and suppressing duplicate rendered frames.
- Implemented session room archival policy on `/exit` via `RELAY_SESSION_ROOM_ARCHIVE_POLICY` with `keep`, `leave`, and `forget` (default) modes.
- Replaced clone-per-session workspace prep with shared bare-mirror + per-session git worktree creation.
- Added reliable worktree cleanup on session stop/exit, including git worktree metadata pruning.

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

- `/exit` cleanup still has one remaining gap: session room archival policy is not implemented yet.
- Copilot backend remains a stub mapping and is not functionally implemented.