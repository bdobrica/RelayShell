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

## 8. Troubleshooting

- If governor fails with missing env vars: verify `.env` exists and required fields are set.
- If codex fails to authenticate: verify `OPENAI_API_KEY` in `.env` and ensure it is listed in `RELAY_CONTAINER_PASSTHROUGH_ENV`.
- If Matrix login/join fails: inspect homeserver logs with `make tuwunel-logs`.
