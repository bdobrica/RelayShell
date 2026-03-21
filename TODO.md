# Matrix Agent Bridge – TODO / Roadmap

## Overview

Build a Go-based system that allows controlling coding agents (Copilot CLI / Codex CLI) through Matrix rooms.

Core components:

* Governor (control plane)
* Matrix bot integration
* Session manager
* Container runner (PTY-backed)
* Git workspace manager
* Agent backends (Copilot / Codex)

---

## Phase 0 – Project Setup

### Repo Setup

* [x] Initialize Go module
* [x] Create project structure:

  * /cmd/governor
  * /internal/matrixbot
  * /internal/sessions
  * /internal/container
  * /internal/gitops
  * /internal/bridge
  * /internal/agents
  * /internal/store

### Tooling

* [x] Setup Makefile
* [x] Setup linting (golangci-lint)
* [x] Setup formatting (gofmt)
* [x] Setup logging library (log/slog)

---

## Phase 1 – Minimal Working Prototype

### Matrix Bot

* [x] Connect to Matrix homeserver
* [x] Join a predefined Governor room
* [x] Listen for messages
* [x] Send messages to room

### Command Parsing

* [x] Implement strict command parser

  * [x] /start repo=<repo> branch=<branch> agent=<agent>
  * [x] /restart
  * [x] /exit
  * [x] /commit
* [x] Reject non-command messages in Governor room

### Session Model

* [x] Define Session struct
* [x] In-memory session store
* [x] Map roomID -> session

### Room Management

* [x] Create new Matrix room per session
* [x] Invite user to room
* [x] Post session metadata message

### Git Workspace (basic)

* [x] Clone repo into temp directory
* [x] Checkout requested branch

### Container Runner (basic)

* [x] Start container for agent
* [x] Mount workspace
* [x] Attach stdin/stdout

### Bridge (basic)

* [x] Forward Matrix messages -> container stdin
* [x] Forward container stdout -> Matrix messages

### Agent Room Handling

* [x] Accept free text messages
* [x] Route to container
* [x] Add session command (`/enter`) to send raw Enter key to container stdin

---

## Phase 2 – PTY + Stability Improvements

### PTY Integration

* [ ] Replace raw pipes with PTY
* [ ] Handle interactive CLI behavior

### Output Handling

* [ ] Strip ANSI sequences
* [ ] Buffer output (300–1000ms)
* [x] Send Matrix typing indicator while output is buffering/waiting flush timeout
* [ ] Chunk messages
* [ ] Prevent message spam

### Error Handling

* [ ] Handle container crashes
* [ ] Handle broken pipes
* [ ] Handle Matrix reconnects

---

## Phase 3 – Session Lifecycle Commands

### Restart

* [ ] Implement /restart

  * [ ] Stop container
  * [ ] Restart container
  * [ ] Preserve workspace

### Exit

* [ ] Implement /exit

  * [ ] Stop container
  * [ ] Delete workspace
  * [ ] Remove session from store
  * [ ] Notify room

### Commit

* [ ] Implement /commit

  * [ ] Generate git diff
  * [ ] Generate commit message (agent or fallback)
  * [ ] git add -A
  * [ ] git commit
  * [ ] Return commit SHA

---

## Phase 4 – Git Improvements

* [ ] Replace clone with git worktrees
* [ ] Support multiple sessions per repo
* [ ] Clean worktree on exit

---

## Phase 5 – Agent Backends

### Codex Backend

* [x] Container image
* [x] CLI entrypoint
* [x] Auth handling

### Copilot Backend

* [ ] Container image
* [ ] CLI setup
* [ ] Auth handling

### Abstraction

* [ ] Define AgentBackend interface
* [ ] Implement both backends

---

## Phase 6 – Persistence

* [ ] Add SQLite/Postgres
* [ ] Persist sessions
* [ ] Restore sessions on restart
* [x] Persist processed Matrix event IDs / checkpoints to prevent duplicate command replay after governor restart

---

## Phase 7 – Security & Isolation

* [ ] Run containers as non-root
* [ ] Limit CPU/memory
* [ ] Restrict network access (optional)
* [ ] Secure secret handling

---

## Phase 8 – UX Improvements

* [ ] Pretty formatting of agent output
* [ ] Code block detection
* [ ] Status messages
* [ ] Session summaries

---

## Phase 9 – Advanced Features

* [ ] Multi-user support
* [ ] Permissions (who can start/kill sessions)
* [ ] Session timeouts
* [ ] Resume sessions
* [ ] Logs/history export

---

## Phase 10 – Observability

* [ ] Structured logging
* [ ] Metrics (sessions, errors, usage)
* [ ] Debug mode

---

## Nice-to-Have

* [ ] Web dashboard
* [ ] GitHub PR integration
* [ ] Auto-push after commit
* [ ] Diff preview before commit

---

## Open Questions

* [ ] How to handle Copilot auth in containers?
* [ ] Should sessions be resumable?
* [ ] How strict should command parsing be?
* [ ] Should agent output be editable or raw?

---

## Milestones

### MVP

* Start session
* Chat with agent
* Restart/Exit

### Beta

* Commit support
* Multiple sessions
* Stable output handling

### Production

* Security hardened
* Persistent sessions
* Multi-user support

---

## Notes

* Prefer PTY over raw pipes
* Keep Governor deterministic
* Avoid sharing workspaces between sessions
* Start simple, iterate fast
