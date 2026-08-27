#!/usr/bin/env bash
# =============================================================================
# HYDRA-UMC-TOOL-CLI - Build and Compile Script
# Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
# GPL-3.0 - see LICENSE
# =============================================================================
# Bumps the version, runs the real test suite, then compiles the Go
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

echo "========================================"
echo " HYDRA-UMC-TOOL-CLI"
echo " Build and Compile Script - bumps the version and compiles the CLI"
echo " Author: JuanenRac (Electro Hobby 3D)"
echo " E-mail: electrohobby3d@gmail.com"
echo " License: GPL-3.0 - see LICENSE"
echo "========================================"
echo ""

echo "[1/4] Bumping version number (odometer bump, see bump_version.py)..."
python3 bump_version.py || exit 1
python3 "$(dirname "$0")/bump_manifest_version.py" --sync || exit 1
echo ""

echo "[2/4] Running the real test suite (go test)..."
( cd src && go vet ./... && go test ./... )
echo "      Done."
echo ""

echo "[3/4] Compiling Go module (src/cmd/hydra-cli)..."
mkdir -p build
BIN_NAME="hydra-cli"
if [ "${OS:-}" = "Windows_NT" ]; then
    BIN_NAME="hydra-cli.exe"
fi
( cd src && go build -o "../build/${BIN_NAME}" ./cmd/hydra-cli )
echo "      Done. Binary: build/${BIN_NAME}"
echo ""

echo "[4/4] Verifying the binary runs..."
"./build/${BIN_NAME}" version
echo ""

echo "========================================"
echo " Build complete. Run ./run.sh to execute the binary again."
echo "========================================"
