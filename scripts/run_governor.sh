#!/usr/bin/env bash
set -euo pipefail

ENV_FILE="${1:-.env}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# shellcheck disable=SC1091
source "$SCRIPT_DIR/dotenv.sh"
load_env_file "$ENV_FILE"

go run ./cmd/governor
