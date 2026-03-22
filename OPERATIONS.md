# RelayShell Operations

This document explains how to run local operations with Makefile targets and how to connect Element Web to a local tuwunel instance.

## 1. Quick Start

1. Copy environment template:

```bash
cp .env.example .env
```

2. Edit `.env` and set at minimum:
- `OPENAI_API_KEY`

3. Build Codex image:

```bash
make build-codex-image
```
3a. Build Copilot image (if you plan to use `agent=copilot`):

```bash
make build-copilot-image
```

4. Start tuwunel:

```bash
make tuwunel-up
```

5. Bootstrap Matrix users and governor room automatically:

```bash
make matrix-bootstrap
```

6. Start RelayShell governor:

```bash
make governor-run
```

## 2. Makefile Targets

- `make tuwunel-up`: starts local tuwunel homeserver with Docker Compose.
- `make tuwunel-down`: stops and removes compose services.
- `make tuwunel-logs`: tails tuwunel logs.
- `make build-codex-image`: builds `relayshell-codex:latest` from `Dockerfile.codex`.
- `make build-copilot-image`: builds `relayshell-copilot:latest` from `Dockerfile.copilot`.
- `make matrix-bootstrap`: registers/logs in bot and human users, creates governor room, invites bot, and writes values into `.env`.
- `make governor-run`: loads variables from `.env` and runs governor.
- `make dev-run`: starts tuwunel, builds codex image, bootstraps matrix users/room, then runs governor in one command.

## 3. Automated Matrix Bootstrap

Run:

```bash
make matrix-bootstrap
```

It will:
- ensure homeserver is reachable
- register or login bot and human users
- create a governor room
- invite the bot to that room
- update `.env` keys used by governor

This automation uses `python3` and only Python standard library modules.

Bootstrap defaults come from `.env`:
- `MATRIX_BOOTSTRAP_BOT_USERNAME`
- `MATRIX_BOOTSTRAP_BOT_PASSWORD`
- `MATRIX_BOOTSTRAP_HUMAN_USERNAME`
- `MATRIX_BOOTSTRAP_HUMAN_PASSWORD`
- `MATRIX_BOOTSTRAP_GOVERNOR_ROOM_NAME`

## 4. Manual Matrix API Flow (Optional)

Use registration token from `.env` (`CONDUWUIT_REGISTRATION_TOKEN`).

```bash
source .env
```

Register bot user:

```bash
curl -sS -X POST "http://localhost:8008/_matrix/client/v3/register" \
  -H "Content-Type: application/json" \
  -d '{
    "username":"relayshell",
    "password":"StrongPass123!",
    "auth":{"type":"m.login.registration_token","token":"'"$CONDUWUIT_REGISTRATION_TOKEN"'"}
  }'
```

Register human user:

```bash
curl -sS -X POST "http://localhost:8008/_matrix/client/v3/register" \
  -H "Content-Type: application/json" \
  -d '{
    "username":"alice",
    "password":"StrongPass123!",
    "auth":{"type":"m.login.registration_token","token":"'"$CONDUWUIT_REGISTRATION_TOKEN"'"}
  }'
```

Copy returned access token for the bot into `.env` as `RELAY_MATRIX_ACCESS_TOKEN`.

## 5. Create Governor Room

Create room using human user token:

```bash
curl -sS -X POST "http://localhost:8008/_matrix/client/v3/createRoom" \
  -H "Authorization: Bearer <ALICE_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"preset":"private_chat","name":"RelayShell Governor"}'
```

Invite bot:

```bash
curl -sS -X POST "http://localhost:8008/_matrix/client/v3/rooms/<ROOM_ID>/invite" \
  -H "Authorization: Bearer <ALICE_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"user_id":"@relayshell:localhost"}'
```

Set `.env` values:
- `RELAY_MATRIX_GOVERNOR_ROOM_ID=<ROOM_ID>`
- `RELAY_ALLOWED_USERS=@alice:localhost`

## 6. Element Web (app.element.io) with Local Tuwunel

Important: `app.element.io` is HTTPS and usually cannot talk directly to `http://localhost:8008` because of browser mixed-content restrictions.

Recommended options:

1. Use Element Desktop for pure local HTTP testing.
2. Use an HTTPS tunnel/proxy for tuwunel and connect Element Web to that HTTPS URL.

### Option A: HTTPS tunnel (quick)

Example with cloudflared:

```bash
cloudflared tunnel --url http://localhost:8008
```

Use the returned `https://...trycloudflare.com` as homeserver URL in Element Web login.

### Option B: Local HTTPS reverse proxy

Run a local TLS reverse proxy (Caddy/nginx) in front of `localhost:8008` and trust its cert locally.
Then use that HTTPS URL in Element Web advanced login.

## 7. Run a Session

1. In governor room, send:

```text
/start repo=https://github.com/<owner>/<repo>.git branch=main agent=codex
```

2. Enter the new session room.
3. Send normal prompts; they are forwarded to Codex inside that session container.
4. Slash commands unknown to RelayShell (for example `/model`) are passed through to the agent in session rooms.
5. Each forwarded message is submitted with Enter automatically. Use `/enter` to send Enter by itself (for prompts like "Press enter to continue").
6. Session commands currently available in session rooms: `/status`, `/restart`, `/commit`, `/exit`, `/enter`.
7. `/commit` stages all workspace changes (`git add -A`), creates a commit with an automatic fallback message, and returns commit SHA.
8. `/exit` applies session room archival policy via `RELAY_SESSION_ROOM_ARCHIVE_POLICY`:
  1. `keep` keeps the bot joined in the session room.
  2. `leave` makes the bot leave the session room.
  3. `forget` makes the bot leave and then forget the room.
9. Commit author identity precedence:
  1. `RELAY_GIT_AUTHOR_NAME` / `RELAY_GIT_AUTHOR_EMAIL` (if set)
  2. host global git config (`git config --global user.name`, `git config --global user.email`)
  3. fallback defaults (`RelayShell`, `relayshell@local`)

## 8. Troubleshooting

- If governor fails with missing env vars: verify `.env` exists and required fields are set.
- If codex fails to authenticate: verify `OPENAI_API_KEY` in `.env` and ensure it is listed in `RELAY_CONTAINER_PASSTHROUGH_ENV`.
- If Matrix login/join fails: inspect homeserver logs with `make tuwunel-logs`.
- Interactive bridge uses PTY by default. Set `RELAY_AGENT_CODEX_COMMAND=codex` and governor auto-normalizes to:
  1. `codex login --with-api-key` using `OPENAI_API_KEY`
  2. `codex --no-alt-screen`
- Copilot backend uses `RELAY_AGENT_COPILOT_COMMAND` (default `copilot`) and attempts non-interactive token bootstrap when `GH_TOKEN` or `GITHUB_TOKEN` is passed through (`copilot auth login --with-token`).
- If PTY startup fails in your environment, governor falls back to pipe mode and logs a warning.
 
- If interactive behavior is still problematic in your environment, fallback to non-interactive mode: `while IFS= read -r line; do [ -z "$line" ] && continue; codex exec --skip-git-repo-check "$line"; done`.
- Bridge flush timing is configurable via `RELAY_BRIDGE_OUTPUT_BATCH_IDLE_MS` (default `300`). Increase it to gather larger redraw batches, or decrease it for lower latency.
- Bridge output uses idle-time debounce flush: each new output chunk resets the timer, and buffered output is sent only after `RELAY_BRIDGE_OUTPUT_BATCH_IDLE_MS` of inactivity.
- `RELAY_BRIDGE_OUTPUT_FLUSH_MAX_MS` sets a hard cap for continuous output streams (default `2000`). Set it to `0` to disable the hard cap and flush only on idle.
- RelayShell sends Matrix messages with both plain `body` and HTML `formatted_body` wrapped in `<pre>` for better alignment of terminal-style output.
- Set `RELAY_BRIDGE_DEBUG_IO=true` to emit debug logs for stdin/stdout/stderr buffers with non-printable bytes rendered as `<HEX>` markers.
- Bridge I/O debug logs are emitted at debug level; set `RELAY_LOG_LEVEL=debug` to see them.
- Enable `RELAY_DEV_IMAGE_TEMPLATES_ENABLED=true` to build a derived worker image per session from generated templates based on detected repository stack (`go`, `python`, `node`, `mixed`).
- `RELAY_DEV_IMAGE_BUILD_TIMEOUT_SEC` controls derived image build timeout (default `600`). On build failure, RelayShell falls back to the base agent image.
- Processed Matrix message events are persisted in SQLite to avoid replay after governor restarts. Configure the DB location via `RELAY_EVENTS_DB_PATH` (default: `${RELAY_WORKSPACE_BASE_DIR}/governor_events.db`).
- Processed event retention is configurable via `RELAY_EVENTS_RETENTION_DAYS` (default: `30`). On governor startup, rows older than this many days are deleted. Set to `0` to disable cleanup.
- RelayShell applies SQLite schema migrations automatically on governor startup using a versioned `schema_migrations` table in the same database.
- Worker runtime isolation controls:
  1. `RELAY_CONTAINER_RUN_AS_NON_ROOT=true` (default) uses host numeric UID:GID, unless overridden with `RELAY_CONTAINER_RUN_AS_USER`.
  2. `RELAY_CONTAINER_CPU_LIMIT` maps to container runtime `--cpus`.
  3. `RELAY_CONTAINER_MEMORY_LIMIT` maps to container runtime `--memory`.
  4. `RELAY_CONTAINER_NETWORK` maps to container runtime `--network` (for example: `none`).
- Secret passthrough variables configured in `RELAY_CONTAINER_PASSTHROUGH_ENV` are injected by name (`-e KEY`) and not rendered as `KEY=value` in runtime command arguments.

## 9. Current Limitations

- PTY is implemented with a pipe-mode fallback if PTY allocation fails in a given runtime environment.
- Session processes are now monitored; unexpected container exits trigger a room notification and session state transition.
- Session lifecycle state is persisted in SQLite and restored on governor restart. RelayShell auto-attempts process restore for resumable states.
- Copilot backend is implemented with dedicated image/command defaults and optional non-interactive token bootstrap.
- Container runtime hardening is currently runtime-flag based (user/CPU/memory/network). Further sandboxing (seccomp/apparmor/capabilities profiles) is not implemented yet.
