#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVER_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$SERVER_ROOT"
go build -o build/battle-ml-sim ./cmd/battle-ml-sim

cd "$SERVER_ROOT/ml"
exec uv run --python 3.12 python -m dice_destiny_ml "$@"
