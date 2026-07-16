#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CLIENT_NATIVE_DIR="${ROOT_DIR}/../dice-and-destiny-client/native"
BUILD_DIR="${ROOT_DIR}/build/darwin-arm64"
ADAPTER_DIR="${ROOT_DIR}/adapters/gdextension"
GODOT_CPP_DIR="${ROOT_DIR}/third_party/godot-cpp"
VENV_DIR="${ROOT_DIR}/.venv"
JOBS="$(sysctl -n hw.ncpu 2>/dev/null || echo 4)"
GODOT_BIN="${GODOT_BIN:-godot}"
FINGERPRINT_FILE="${BUILD_DIR}/native-inputs.sha256"
CLIENT_GO_LIBRARY="${CLIENT_NATIVE_DIR}/libbattle_go_authority.dylib"
CLIENT_CPP_FRAMEWORK="${CLIENT_NATIVE_DIR}/libbattle_authority.macos.template_debug.framework"
CLIENT_CPP_LIBRARY="${CLIENT_CPP_FRAMEWORK}/libbattle_authority.macos.template_debug"

if_needed=0
for argument in "$@"; do
  case "${argument}" in
    --if-needed)
      if_needed=1
      ;;
    *)
      echo "unknown build_native.sh argument: ${argument}" >&2
      exit 2
      ;;
  esac
done

# SCons and the final framework publication use fixed workspace-local paths.
# Serialize duplicate build requests in the same workspace while allowing
# builds in other worktrees to proceed independently.
if [[ "${DICE_AND_DESTINY_NATIVE_BUILD_LOCKED:-0}" != "1" ]] && command -v lockf >/dev/null 2>&1; then
  export DICE_AND_DESTINY_NATIVE_BUILD_LOCKED=1
  exec lockf -k "${ROOT_DIR}/.build_native.lock" "$0" "$@"
fi

mkdir -p "${BUILD_DIR}" "${CLIENT_NATIVE_DIR}"

hash_stream() {
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 | awk '{print $1}'
    return
  fi
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum | awk '{print $1}'
    return
  fi
  echo "build_native.sh requires shasum or sha256sum" >&2
  return 1
}

native_input_paths() {
  (
    cd "${ROOT_DIR}"
    {
      find internal adapters/gdextension/go_export \
        -type f -name '*.go' ! -name '*_test.go' -print
      find adapters/gdextension/src adapters/gdextension/include \
        -type f \( -name '*.cpp' -o -name '*.h' \) -print
      printf '%s\n' \
        go.mod \
        go.sum \
        adapters/gdextension/SConstruct \
        scripts/build_native.sh
      if [[ -f adapters/gdextension/extension_api.json ]]; then
        printf '%s\n' adapters/gdextension/extension_api.json
      fi
    } | LC_ALL=C sort -u
  )
}

native_input_fingerprint() {
  local godot_cpp_commit godot_cpp_diff git_prefix godot_version go_version
  local input_hashes input_paths

  input_paths="$(native_input_paths)" || return 1
  git_prefix="$(git -C "${ROOT_DIR}" rev-parse --show-prefix)" || return 1
  input_hashes="$(
    printf '%s\n' "${input_paths}" | \
      sed "s#^#${git_prefix}#" | \
      git -C "${ROOT_DIR}/.." hash-object --stdin-paths
  )" || return 1
  go_version="$(go version)" || return 1
  godot_version="$("${GODOT_BIN}" --version 2>/dev/null)" || return 1

  if [[ -d "${GODOT_CPP_DIR}/.git" ]]; then
    godot_cpp_commit="$(git -C "${GODOT_CPP_DIR}" rev-parse HEAD)" || return 1
    godot_cpp_diff="$(git -C "${GODOT_CPP_DIR}" diff --binary HEAD)" || return 1
  else
    godot_cpp_commit="missing"
    godot_cpp_diff=""
  fi

  {
    printf '%s\n' \
      'build_tags=scenario_tools,snapshot_tools,history_tools' \
      'buildmode=c-shared' \
      'platform=macos' \
      'arch=arm64' \
      'target=template_debug'
    printf 'go-version=%s\n' "${go_version}"
    printf 'godot-version=%s\n' "${godot_version}"

    printf '%s\n' 'native-input-paths-begin'
    printf '%s\n' "${input_paths}"
    printf '%s\n' 'native-input-paths-end' 'native-input-hashes-begin'
    printf '%s\n' "${input_hashes}"
    printf '%s\n' 'native-input-hashes-end'

    printf 'godot-cpp-commit=%s\n' "${godot_cpp_commit}"
    printf '%s\n' 'godot-cpp-diff-begin'
    printf '%s\n' "${godot_cpp_diff}"
    printf '%s\n' 'godot-cpp-diff-end'
  } | hash_stream
}

inputs_fingerprint="$(native_input_fingerprint)"
if [[ "${if_needed}" == "1" ]] && \
   [[ -f "${CLIENT_GO_LIBRARY}" ]] && \
   [[ -f "${CLIENT_CPP_LIBRARY}" ]] && \
   [[ -f "${FINGERPRINT_FILE}" ]] && \
   [[ "$(<"${FINGERPRINT_FILE}")" == "${inputs_fingerprint}" ]]; then
  echo "Native artifacts are up to date."
  exit 0
fi

if [[ ! -f "${GODOT_CPP_DIR}/SConstruct" ]]; then
  mkdir -p "${ROOT_DIR}/third_party"
  git clone --depth 1 https://github.com/godotengine/godot-cpp.git "${GODOT_CPP_DIR}"
fi

(
  cd "${ROOT_DIR}"
  go build \
    -tags "scenario_tools snapshot_tools history_tools" \
    -buildmode=c-shared \
    -o "${BUILD_DIR}/libbattle_go_authority.dylib" \
    ./adapters/gdextension/go_export
)

cp "${BUILD_DIR}/libbattle_go_authority.dylib" "${CLIENT_NATIVE_DIR}/"

# Go's Darwin linker emits an ad-hoc signature, but copying the dylib can leave
# macOS with a stale executable-page signature cache. Re-sign the exact client
# artifact Godot will dlopen so hardened runtime validation remains reliable.
if command -v codesign >/dev/null 2>&1; then
  codesign --force --sign - "${CLIENT_NATIVE_DIR}/libbattle_go_authority.dylib"
fi

if [[ ! -x "${VENV_DIR}/bin/scons" ]]; then
  python3 -m venv "${VENV_DIR}"
  "${VENV_DIR}/bin/python" -m pip install --upgrade pip scons
fi

if [[ ! -f "${ADAPTER_DIR}/extension_api.json" ]]; then
  (
    cd "${ADAPTER_DIR}"
    "${GODOT_BIN}" --headless --dump-extension-api
  )
fi

(
  cd "${ADAPTER_DIR}"
  "${VENV_DIR}/bin/scons" \
    platform=macos \
    arch=arm64 \
    target=template_debug \
    custom_api_file=extension_api.json \
    -j"${JOBS}"
)

rm -rf "${CLIENT_NATIVE_DIR}/libbattle_authority.macos.template_debug.framework"
cp -R "${ROOT_DIR}/build/gdextension/libbattle_authority.macos.template_debug.framework" "${CLIENT_NATIVE_DIR}/"

if command -v codesign >/dev/null 2>&1; then
  codesign --force --deep --sign - "${CLIENT_NATIVE_DIR}/libbattle_authority.macos.template_debug.framework"
fi

# Only publish the fingerprint after every build and signing step succeeds.
# Recalculate it because a first build may clone godot-cpp or generate the
# extension API, both of which are native build inputs.
inputs_fingerprint="$(native_input_fingerprint)"
fingerprint_temp="${FINGERPRINT_FILE}.tmp.$$"
printf '%s\n' "${inputs_fingerprint}" > "${fingerprint_temp}"
mv "${fingerprint_temp}" "${FINGERPRINT_FILE}"
