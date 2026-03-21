# RelayShell

> Continue your coding session from anywhere — through chat.

RelayShell is a lightweight, Matrix-powered bridge that lets you interact with terminal-based coding agents (like Codex CLI or Copilot CLI) remotely, as if they were local shells.

It’s designed for one simple goal:

> **Pick up your work anytime, from anywhere — even during a lunch break.**

---

## ✨ What is RelayShell?

RelayShell turns a Matrix room into a **live terminal session** backed by a containerized coding agent.

* You send messages → they go to the agent’s stdin
* The agent responds → output is streamed back to the room
* Each session runs in isolation
* You can restart, commit, or exit at any time

It’s like having a **remote dev shell that lives in chat**.

---

## 🧠 Core Idea

RelayShell is not trying to be a full orchestration platform.

It’s a **personal tool** that helps you:

* Resume work quickly
* Use idle time effectively
* Interact with coding agents without opening your full dev environment

Think of it as:

> **“session continuity for coding, powered by chat.”**

---

## 🏗️ How It Works

1. You send a command in a Matrix “Governor” room
2. RelayShell creates a new session
3. A dedicated Matrix room is created
4. A Git workspace is prepared (repo + branch)
5. A container starts with your agent (Codex/Copilot)
6. Messages in the room are piped to the agent
7. Output is streamed back into the room

---

## 🚀 Features

* 💬 **Chat-driven workflow** — control everything from Matrix
* 🧵 **One room per session** — clean separation of work
* 🐳 **Container isolation** — safe parallel runs
* 🔁 **Live shell bridge** — stdin/stdout over chat
* 🌿 **Git integration** — work on real branches
* ⚙️ **Deterministic commands** — no ambiguity
* ⚡ **Fast context switching** — resume instantly

---

## 💬 Usage

### Governor Room

Start a new session:

```text
/start repo=my-api branch=feature/auth agent=codex
```

Other commands:

```text
/list
/status <session-id>
/kill <session-id>
/help
```

---

### Session Room

Once created, the session room becomes your remote shell.

#### Send normal messages

```text
Add JWT authentication middleware
```

These go directly to the agent.

#### Control commands

```text
/restart   # restart agent session
/commit    # planned (currently not implemented)
/exit      # stop session
/status    # show session state
```

---

## 🔁 Session Lifecycle

Each session goes through:

* `creating`
* `preparing_workspace`
* `creating_room`
* `starting_container`
* `running`
* `restarting`
* `stopping`
* `exited`
* `failed`

---

## 🧪 Example Workflow

1. Start a session:

```text
/start repo=my-api branch=feat/login agent=codex
```

2. Enter the created room

3. Work normally:

```text
Implement login endpoint with JWT
```

4. Commit changes:

```text
/commit
```

5. Exit session:

```text
/exit
```

---

## ⚙️ Setup

Current status: core session flow is functional for `agent=codex`; some lifecycle and stability tasks are still in progress.

### Requirements

* Go (1.22+)
* Docker or Podman
* Matrix account
* Git

### Build

```bash
git clone https://github.com/<your-username>/relayshell
cd relayshell
go build -o relayshell ./cmd/governor
```

### Local Development (Phase 0)

```bash
# Install local tooling (one-time)
make install-tools

# Format, test, lint, and build
make fmt
make test
make lint
make build
```

### Build Codex Worker Image

```bash
make build-codex-image
```

### Run Governor (Phase 1 Prototype)

```bash
export RELAY_MATRIX_HOMESERVER="http://localhost:8008"
export RELAY_MATRIX_USER_ID="@relayshell:localhost"
export RELAY_MATRIX_ACCESS_TOKEN="YOUR_ACCESS_TOKEN"
export RELAY_MATRIX_GOVERNOR_ROOM_ID="!governorRoomId:localhost"
export RELAY_LOG_LEVEL="info"

# Optional overrides
export RELAY_WORKSPACE_BASE_DIR="/tmp/relayshell"
export RELAY_EVENTS_DB_PATH="/tmp/relayshell/governor_events.db"
export RELAY_EVENTS_RETENTION_DAYS="30"
export RELAY_CONTAINER_RUNTIME="docker"
export RELAY_CONTAINER_IMAGE="alpine:3.20"
export RELAY_AGENT_CODEX_IMAGE="relayshell-codex:latest"
export RELAY_AGENT_CODEX_COMMAND="codex"
export RELAY_BRIDGE_OUTPUT_BATCH_IDLE_MS="300"
export RELAY_BRIDGE_OUTPUT_FLUSH_MAX_MS="2000"
export RELAY_BRIDGE_DEBUG_IO="false"
export RELAY_CONTAINER_PASSTHROUGH_ENV="OPENAI_API_KEY,OPENAI_BASE_URL,OPENAI_ORG_ID,OPENAI_PROJECT"
export RELAY_ALLOWED_USERS="@yourUser:localhost"

# Required for Codex
export OPENAI_API_KEY="YOUR_OPENAI_API_KEY"

make run
```

`codex` sessions run inside the dedicated Codex container image and execute the configured Codex command.
`copilot` is currently a stub mapping (`cat`) and is not a functional backend yet.

### Config Example

```yaml
matrix:
  homeserver: "https://matrix.org"
  user_id: "@relayshell:matrix.org"
  access_token: "YOUR_TOKEN"
  governor_room_id: "!yourRoomId:matrix.org"

workspace:
  base_dir: "/tmp/relayshell"

containers:
  runtime: "docker"
  default_image: "relayshell-agent:latest"

security:
  allowed_users:
    - "@yourUser:matrix.org"
```

---

## 🔐 Security Notes

* Each session runs in its own container
* Workspaces are isolated per session
* Only authorized users can control RelayShell
* Secrets must be injected carefully

---

## 🧱 Roadmap

Completed items are tracked in `CHANGELOG.md`.

### In Progress

* [ ] True PTY integration (currently raw stdio with terminal-command workaround)
* [ ] Container crash detection and automatic session-state handling
* [ ] Matrix reconnect backoff/recovery strategy
* [ ] `/commit` implementation
* [ ] `/exit` workspace cleanup

### Planned

* [ ] Git worktree optimization and multi-session repo handling
* [ ] Session persistence and restore on governor restart
* [ ] Fully functional Copilot backend
* [ ] Security hardening (non-root containers, limits, secret handling)
* [ ] UX improvements (status messages, summaries, formatting)
* [ ] Observability (metrics, stronger structured logging)

---

## 🧠 Philosophy

RelayShell is built around a few simple ideas:

* Coding should not be tied to a single machine
* Small time windows are valuable
* Agents should behave like tools, not magic
* Chat can be a powerful control interface

---

## 🤝 Contributing

Ideas, experiments, and contributions are welcome.

---

## 📄 License

[Apache 2.0](./LICENSE)
