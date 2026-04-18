#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CLIENT_NATIVE_DIR="${ROOT_DIR}/../dice-and-destiny-client/native"
BUILD_DIR="${ROOT_DIR}/build/darwin-arm64"
ADAPTER_DIR="${ROOT_DIR}/adapters/gdextension"
GODOT_CPP_DIR="${ROOT_DIR}/third_party/godot-cpp"
VENV_DIR="${ROOT_DIR}/.venv"
JOBS="$(sysctl -n hw.ncpu 2>/dev/null || echo 4)"

mkdir -p "${BUILD_DIR}" "${CLIENT_NATIVE_DIR}"

if [[ ! -f "${GODOT_CPP_DIR}/SConstruct" ]]; then
  mkdir -p "${ROOT_DIR}/third_party"
  git clone --depth 1 https://github.com/godotengine/godot-cpp.git "${GODOT_CPP_DIR}"
fi

go build \
  -buildmode=c-shared \
  -o "${BUILD_DIR}/libbattle_go_authority.dylib" \
  "${ROOT_DIR}/adapters/gdextension/go_export"

cp "${BUILD_DIR}/libbattle_go_authority.dylib" "${CLIENT_NATIVE_DIR}/"

if [[ ! -x "${VENV_DIR}/bin/scons" ]]; then
  python3 -m venv "${VENV_DIR}"
  "${VENV_DIR}/bin/python" -m pip install --upgrade pip scons
fi

if [[ ! -f "${ADAPTER_DIR}/extension_api.json" ]]; then
  (
    cd "${ADAPTER_DIR}"
    godot --headless --dump-extension-api
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
