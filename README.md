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
/commit    # create a git commit from workspace changes
/tree      # show workspace tree
/diff      # show changed files summary (+added/-removed)
/diff <relative-file>  # show file patch
/push      # push current branch commits to remote using configured SSH key
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
* `committing`
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

Current status: core session flow is functional for `agent=codex`, including stack-aware derived dev image builds (opt-in) and persisted session lifecycle restore after governor restart; some backend and hardening tasks are still in progress.

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

### Build Copilot Worker Image

```bash
make build-copilot-image
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
export RELAY_CONTAINER_RUN_AS_NON_ROOT="true"
export RELAY_CONTAINER_RUN_AS_USER=""
export RELAY_CONTAINER_CPU_LIMIT="1.5"
export RELAY_CONTAINER_MEMORY_LIMIT="2g"
export RELAY_CONTAINER_NETWORK=""
export RELAY_AGENT_CODEX_IMAGE="relayshell-codex:latest"
export RELAY_AGENT_CODEX_COMMAND="codex"
export RELAY_AGENT_COPILOT_IMAGE="relayshell-copilot:latest"
export RELAY_AGENT_COPILOT_COMMAND="copilot"
export RELAY_BRIDGE_OUTPUT_BATCH_IDLE_MS="300"
export RELAY_BRIDGE_OUTPUT_FLUSH_MAX_MS="2000"
export RELAY_BRIDGE_DEBUG_IO="false"
export RELAY_SESSION_ROOM_ARCHIVE_POLICY="forget"
export RELAY_DEV_IMAGE_TEMPLATES_ENABLED="false"
export RELAY_DEV_IMAGE_BUILD_TIMEOUT_SEC="600"
export RELAY_GIT_AUTHOR_NAME=""
export RELAY_GIT_AUTHOR_EMAIL=""
export RELAY_GIT_PUSH_SSH_KEY_PATH=""
export RELAY_GIT_PUSH_SSH_PRIVATE_KEY=""
export RELAY_GIT_PUSH_REMOTE="origin"
export RELAY_CONTAINER_PASSTHROUGH_ENV="OPENAI_API_KEY,OPENAI_BASE_URL,OPENAI_ORG_ID,OPENAI_PROJECT,GH_TOKEN,GITHUB_TOKEN"
export RELAY_ALLOWED_USERS="@yourUser:localhost"

# Required for Codex
export OPENAI_API_KEY="YOUR_OPENAI_API_KEY"

make run
```

`codex` sessions run inside the dedicated Codex container image and execute the configured Codex command.
`copilot` sessions run inside the dedicated Copilot container image and execute the configured Copilot command.
When `GH_TOKEN` or `GITHUB_TOKEN` is passed through, RelayShell attempts non-interactive `copilot auth login --with-token` before launching the Copilot command.

When `RELAY_DEV_IMAGE_TEMPLATES_ENABLED=true`, RelayShell builds a derived image using `internal/devimage/templates/Dockerfile.dev.tmpl` and toggles language install paths via Docker build args (`BASE_IMAGE`, `ENABLE_GO`, `ENABLE_PYTHON`, `ENABLE_NODEJS`) based on detected repository stack.

`/commit` author identity precedence:
1. `RELAY_GIT_AUTHOR_NAME` / `RELAY_GIT_AUTHOR_EMAIL` (if set)
2. host global git config (`user.name`, `user.email`)
3. fallback defaults (`RelayShell`, `relayshell@local`)

`/push` SSH key precedence:
1. `RELAY_GIT_PUSH_SSH_KEY_PATH` (path to private key file)
2. `RELAY_GIT_PUSH_SSH_PRIVATE_KEY` (inline private key content; supports escaped `\n` newlines)

If both are set, `RELAY_GIT_PUSH_SSH_KEY_PATH` is used.

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

* [ ] Python requirements merge/install for template-driven containers
* [ ] Better startup/progress notifications for long-running session initialization

### Planned

* [x] Git worktree optimization and multi-session repo handling
* [x] Template-driven dev container builds (initial: stack detection + baseline language toolchains + Docker ARG toggles for stack paths)
* [x] Session persistence and restore on governor restart
* [ ] Fully functional Copilot backend
* [x] Security hardening (non-root containers, limits, secret handling)
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
