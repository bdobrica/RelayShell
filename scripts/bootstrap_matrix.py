#!/usr/bin/env python3
"""Bootstrap local Matrix users and governor room for RelayShell integration tests."""

from __future__ import annotations

import json
import pathlib
import re
import sys
import urllib.error
import urllib.parse
import urllib.request
from typing import Dict, Tuple


def load_dotenv(path: pathlib.Path) -> Dict[str, str]:
    env: Dict[str, str] = {}
    for raw_line in path.read_text(encoding="utf-8").splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, value = line.split("=", 1)
        key = key.strip()
        value = value.strip()
        if (value.startswith('"') and value.endswith('"')) or (
            value.startswith("'") and value.endswith("'")
        ):
            value = value[1:-1]
        env[key] = value
    return env


def write_dotenv(path: pathlib.Path, updates: Dict[str, str]) -> None:
    lines = path.read_text(encoding="utf-8").splitlines()
    found = {key: False for key in updates}

    for i, line in enumerate(lines):
        for key, value in updates.items():
            if re.match(rf"^\s*{re.escape(key)}\s*=", line):
                lines[i] = f"{key}={value}"
                found[key] = True

    for key, value in updates.items():
        if not found[key]:
            lines.append(f"{key}={value}")

    path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def post_json(url: str, payload: Dict[str, object], token: str | None = None) -> Dict[str, object]:
    data = json.dumps(payload).encode("utf-8")
    request = urllib.request.Request(url=url, data=data, method="POST")
    request.add_header("Content-Type", "application/json")
    if token:
        request.add_header("Authorization", f"Bearer {token}")

    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            body = response.read().decode("utf-8")
    except urllib.error.HTTPError as err:
        body = err.read().decode("utf-8") if err.fp else ""
        raise RuntimeError(f"POST {url} failed ({err.code}): {body}") from err

    try:
        return json.loads(body)
    except json.JSONDecodeError as err:
        raise RuntimeError(f"Invalid JSON response from {url}: {body}") from err


def get_json(url: str) -> Dict[str, object]:
    request = urllib.request.Request(url=url, method="GET")
    try:
        with urllib.request.urlopen(request, timeout=15) as response:
            body = response.read().decode("utf-8")
    except urllib.error.HTTPError as err:
        body = err.read().decode("utf-8") if err.fp else ""
        raise RuntimeError(f"GET {url} failed ({err.code}): {body}") from err

    try:
        return json.loads(body)
    except json.JSONDecodeError as err:
        raise RuntimeError(f"Invalid JSON response from {url}: {body}") from err


def register_or_login(
    homeserver: str,
    reg_token: str,
    username: str,
    password: str,
) -> Tuple[str, str]:
    register_payload = {
        "username": username,
        "password": password,
        "auth": {"type": "m.login.registration_token", "token": reg_token},
    }

    try:
        response = post_json(f"{homeserver}/_matrix/client/v3/register", register_payload)
        access_token = str(response.get("access_token", ""))
        user_id = str(response.get("user_id", ""))
        if access_token and user_id:
            return access_token, user_id
    except RuntimeError:
        pass

    login_payload = {
        "type": "m.login.password",
        "identifier": {"type": "m.id.user", "user": username},
        "password": password,
    }
    response = post_json(f"{homeserver}/_matrix/client/v3/login", login_payload)

    access_token = str(response.get("access_token", ""))
    user_id = str(response.get("user_id", ""))
    if not access_token or not user_id:
        raise RuntimeError(f"Failed to register/login user '{username}': {response}")
    return access_token, user_id


def main() -> int:
    env_file = pathlib.Path(sys.argv[1] if len(sys.argv) > 1 else ".env")
    if not env_file.exists():
        print(f"Missing {env_file}")
        return 1

    env = load_dotenv(env_file)

    port = env.get("CONDUWUIT_PORT", "8008")
    homeserver = env.get("RELAY_MATRIX_HOMESERVER", f"http://localhost:{port}")
    reg_token = env.get("CONDUWUIT_REGISTRATION_TOKEN", "")

    bot_username = env.get("MATRIX_BOOTSTRAP_BOT_USERNAME", "relayshell")
    bot_password = env.get("MATRIX_BOOTSTRAP_BOT_PASSWORD", "StrongPass123!")
    human_username = env.get("MATRIX_BOOTSTRAP_HUMAN_USERNAME", "alice")
    human_password = env.get("MATRIX_BOOTSTRAP_HUMAN_PASSWORD", "StrongPass123!")
    room_name = env.get("MATRIX_BOOTSTRAP_GOVERNOR_ROOM_NAME", "RelayShell Governor")

    versions = get_json(f"{homeserver}/_matrix/client/versions")
    if "versions" not in versions:
        raise RuntimeError(f"Matrix homeserver not reachable at {homeserver}")

    print(f"Bootstrapping Matrix users on {homeserver}...")

    bot_token, bot_user_id = register_or_login(homeserver, reg_token, bot_username, bot_password)
    human_token, human_user_id = register_or_login(homeserver, reg_token, human_username, human_password)

    room_resp = post_json(
        f"{homeserver}/_matrix/client/v3/createRoom",
        {"preset": "private_chat", "name": room_name},
        token=human_token,
    )
    room_id = str(room_resp.get("room_id", ""))
    if not room_id:
        raise RuntimeError(f"Failed to create governor room: {room_resp}")

    encoded_room_id = urllib.parse.quote(room_id, safe="")
    invite_resp = post_json(
        f"{homeserver}/_matrix/client/v3/rooms/{encoded_room_id}/invite",
        {"user_id": bot_user_id},
        token=human_token,
    )
    if "errcode" in invite_resp:
        print(f"Invite response: {invite_resp}")

    updates = {
        "RELAY_MATRIX_HOMESERVER": homeserver,
        "RELAY_MATRIX_USER_ID": bot_user_id,
        "RELAY_MATRIX_ACCESS_TOKEN": bot_token,
        "RELAY_MATRIX_GOVERNOR_ROOM_ID": room_id,
        "RELAY_ALLOWED_USERS": human_user_id,
    }
    write_dotenv(env_file, updates)

    print("")
    print(f"Bootstrap complete. Updated {env_file} with:")
    print(f"- RELAY_MATRIX_USER_ID={bot_user_id}")
    print("- RELAY_MATRIX_ACCESS_TOKEN=<redacted>")
    print(f"- RELAY_MATRIX_GOVERNOR_ROOM_ID={room_id}")
    print(f"- RELAY_ALLOWED_USERS={human_user_id}")
    print("")
    print("You can now run: make governor-run")

    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except RuntimeError as err:
        print(str(err), file=sys.stderr)
        raise SystemExit(1)
