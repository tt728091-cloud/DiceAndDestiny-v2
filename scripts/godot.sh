#!/usr/bin/env bash
set -euo pipefail

REPOSITORY_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CLIENT_ROOT="${REPOSITORY_ROOT}/dice-and-destiny-client"
SERVER_ROOT="${REPOSITORY_ROOT}/dice-and-destiny-server"
WORKSPACE_RUNTIME_ROOT="${CLIENT_ROOT}/.godot/runtime"
WORKSPACE_STATE_ROOT="${WORKSPACE_RUNTIME_ROOT}/workspace"
LOG_ROOT="${WORKSPACE_RUNTIME_ROOT}/logs"
GODOT_BIN="${GODOT_BIN:-godot}"

mkdir -p "${WORKSPACE_STATE_ROOT}" "${LOG_ROOT}" "${WORKSPACE_RUNTIME_ROOT}/runs"

if [[ ! -f "${CLIENT_ROOT}/native/libbattle_go_authority.dylib" ]] || \
   [[ ! -d "${CLIENT_ROOT}/native/libbattle_authority.macos.template_debug.framework" ]]; then
  "${SERVER_ROOT}/scripts/build_native.sh"
fi

# A brand-new worktree has no Godot global-class cache yet. Import it once
# under a workspace-local lock so the first direct game/test run is as reliable
# as later runs, even if two commands start at the same time.
ensure_imported() {
  if [[ -f "${CLIENT_ROOT}/.godot/global_script_class_cache.cfg" ]]; then
    return
  fi
  import_log="$(mktemp "${LOG_ROOT}/godot-import.XXXXXX")"
  "${GODOT_BIN}" \
    --headless \
    --editor \
    --path "${CLIENT_ROOT}" \
    --log-file "${import_log}" \
    --import \
    --quit
}

if command -v lockf >/dev/null 2>&1; then
  exec 9>"${WORKSPACE_RUNTIME_ROOT}/import.lock"
  lockf 9
  ensure_imported
  exec 9>&-
else
  ensure_imported
fi

isolated_run=0
for argument in "$@"; do
  case "${argument}" in
    --script|-s)
      isolated_run=1
      ;;
    --path)
      echo "scripts/godot.sh owns --path; pass only Godot arguments after the project path" >&2
      exit 2
      ;;
  esac
done

run_root=""
cleanup() {
  if [[ -n "${run_root}" && -d "${run_root}" ]]; then
    rm -rf "${run_root}"
  fi
}
trap cleanup EXIT INT TERM

if [[ "${isolated_run}" == "1" ]]; then
  run_root="$(mktemp -d "${WORKSPACE_RUNTIME_ROOT}/runs/run.XXXXXX")"
  runtime_root="${run_root}/client"
  state_root="${run_root}/server"
else
  runtime_root="${WORKSPACE_STATE_ROOT}/client"
  state_root="${WORKSPACE_STATE_ROOT}/server"
fi

mkdir -p \
  "${runtime_root}" \
  "${state_root}/battles" \
  "${state_root}/scenarios" \
  "${state_root}/snapshots" \
  "${state_root}/history"

export DICE_AND_DESTINY_RUNTIME_ROOT="${runtime_root}"
export DICE_AND_DESTINY_CONTENT_ROOT="${SERVER_ROOT}/content"
export DICE_AND_DESTINY_RUN_STATE_ROOT="${SERVER_ROOT}/save/run_players"
export DICE_AND_DESTINY_BATTLE_STATE_ROOT="${state_root}/battles"
export DICE_AND_DESTINY_SCENARIO_STATE_ROOT="${state_root}/scenarios"
export DICE_AND_DESTINY_SNAPSHOT_STATE_ROOT="${state_root}/snapshots"
export DICE_AND_DESTINY_HISTORY_STATE_ROOT="${state_root}/history"
export DICE_AND_DESTINY_SCENARIO_ROOT="${SERVER_ROOT}/scenarios"

# Port zero asks the OS for a free ephemeral port. Always replace inherited
# values so a shell-level fixed port cannot accidentally reintroduce collisions.
export DICE_AND_DESTINY_INSPECTOR_PORT=0

log_file="$(mktemp "${LOG_ROOT}/godot.XXXXXX")"

godot_arguments=(--path "${CLIENT_ROOT}" --log-file "${log_file}")

set +e
"${GODOT_BIN}" "${godot_arguments[@]}" "$@"
status=$?
set -e
exit "${status}"
