#!/usr/bin/env bash
# HYDRA_UMC_SCRIPT_STANDARD_HEADER_BEGIN
# *****************************************************************************
# Project   : HYDRA-UMC-TOOL-CLI
# Script    : build.sh
# Purpose   : Incremental project build, verification and packaging workflow.
# Author    : JuanenRac (Electro Hobby 3D)
# Email     : electrohobby3d@gmail.com
# Copyright : (C) 2026 JuanenRac
# License   : GPL-3.0 - see LICENSE
# *****************************************************************************
# HYDRA_UMC_SCRIPT_STANDARD_HEADER_END
# HYDRA_UMC_SCRIPT_STANDARD_BANNER_BEGIN
printf '\n*******************************************************************************\n'
printf '%s\n' "* HYDRA-UMC-TOOL-CLI - build.sh"
printf '%s\n' "* Mode      : INCREMENTAL BUILD"
printf '%s\n' "* Author    : JuanenRac (Electro Hobby 3D)"
printf '%s\n' "* Email     : electrohobby3d@gmail.com"
printf '%s\n' "* Copyright : (C) 2026 JuanenRac"
printf '%s\n' "* License   : GPL-3.0 - see LICENSE"
printf '%s\n' "* ------------------------------------------------------------------------- *"
printf '%s\n' "* 1. Read the project version (building never changes it; use the explicit bump command)."
printf '%s\n' "* 2. Run this project's declared build, verification and packaging commands."
printf '%s\n' "* 3. Report the result and keep an interactive terminal open."
printf '%s\n' "*******************************************************************************"
printf '\n'
# HYDRA_UMC_SCRIPT_STANDARD_BANNER_END
# Runs the real test suite, then compiles the Go
# module in src/ into build/hydra-cli(.exe). Run this before run.sh.
#
# Usage:
#   chmod +x build.sh   (one-time)
#   ./build.sh
set -euo pipefail
cd "$(dirname "$0")"

# Keep the window open if this was double-clicked instead of run from an
# already-open terminal - fires on success AND on a `set -e` early exit
# alike, but only prompts when stdin is actually a terminal (never in
# CI/piped/non-interactive runs).
trap '[ -t 0 ] && read -r -p "Press Enter to close..." _' EXIT
# HYDRA_UMC_SCRIPT_STANDARD_VERSION_STEP
printf '%s\n' "[1/5] Reading the project version (building never changes it)..."
HYDRA_UMC_VERSION_BUILT="$(python3 -c 'import json, pathlib, sys; print(json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["version"])' "$(dirname "$0")/hydra-umc.project.json")"
printf '%s\n' "      Building v${HYDRA_UMC_VERSION_BUILT:-unknown} (building never changes the version)"
echo
echo ""

echo "[2/5] Running the real test suite (go test)..."
( cd src && go vet ./... && go test ./... )
echo "      Done."
echo ""

echo "[3/5] Compiling Go module (src/cmd/hydra-cli)..."
mkdir -p build
BIN_NAME="hydra-cli"
if [ "${OS:-}" = "Windows_NT" ]; then
    BIN_NAME="hydra-cli.exe"
fi
( cd src && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.Version=${HYDRA_UMC_VERSION_BUILT}" -o "../build/${BIN_NAME}" ./cmd/hydra-cli )
echo "      Done. Binary: build/${BIN_NAME}"
echo ""

echo "[4/5] Verifying the binary runs..."
"./build/${BIN_NAME}" version
echo ""

# C12 (this repo's own real packaging gap): until now this script only
# ever produced ONE native binary, for running/testing on THIS machine -
# nothing here ever cross-compiled a real release for the other
# platforms an operator's own workstation (or the CM5 itself) might
# actually be. Go's own cross-compilation is just GOOS/GOARCH env vars,
# no separate toolchain needed - real, not aspirational.
echo "[5/5] Packaging cross-platform release binaries..."
RELEASE_DIR="build/release"
rm -rf "$RELEASE_DIR"
mkdir -p "$RELEASE_DIR"
# linux/arm64 is the CM5's own real architecture; the other four cover
# an operator's own workstation (Windows/Linux/macOS, both Apple
# Silicon and Intel).
for target in "linux amd64" "linux arm64" "windows amd64" "darwin amd64" "darwin arm64"; do
    read -r goos goarch <<< "$target"
    out_name="hydra-cli"
    [ "$goos" = "windows" ] && out_name="hydra-cli.exe"
    out_dir="$RELEASE_DIR/${goos}-${goarch}"
    mkdir -p "$out_dir"
    ( cd src && CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -trimpath -ldflags "-s -w -X main.Version=${HYDRA_UMC_VERSION_BUILT}" -o "../${out_dir}/${out_name}" ./cmd/hydra-cli )
    echo "      Built: ${out_dir}/${out_name}"
done
# A real, verifiable checksum per artifact - the same real integrity
# guarantee HYDRA-UMC-OS-REBUILDER's own base-image download already
# relies on for a real published release, applied here to this
# project's own output instead of someone else's.
(
    cd "$RELEASE_DIR"
    if command -v sha256sum >/dev/null 2>&1; then
        find . -type f \( -name "hydra-cli" -o -name "hydra-cli.exe" \) -exec sha256sum {} \; > SHA256SUMS
    else
        shasum -a 256 $(find . -type f \( -name "hydra-cli" -o -name "hydra-cli.exe" \)) > SHA256SUMS
    fi
)
echo "      Done. Checksums: ${RELEASE_DIR}/SHA256SUMS"
echo ""

echo "========================================"
echo " Build complete. Run ./run.sh to execute the binary again."
echo " Cross-platform release artifacts: ${RELEASE_DIR}/"
echo "========================================"
printf '%s\n' "Built v${HYDRA_UMC_VERSION_BUILT:-unknown}. Building never changes the version; to raise it run: python tools/hydra_project_tool.py bump"
