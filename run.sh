#!/usr/bin/env bash
# =============================================================================
# HYDRA-UMC-TOOL-CLI - Run Script
# Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
# GPL-3.0 - see LICENSE
# =============================================================================
# Runs the compiled binary. Run build.sh first. Forwards any arguments
# through to hydra-cli (e.g. "./run.sh status --server http://host:3000").
set -uo pipefail  # no -e: we need to reach the trap below even if the process exits non-zero
cd "$(dirname "$0")"

# Keep the window open if this was double-clicked instead of run from an
# already-open terminal - matters most for a bare double-click (real
# output that would otherwise flash-close before it's readable); running
# from an existing terminal with real arguments is unaffected either way.
# Only prompts when stdin is actually a terminal (never in CI/piped/
# non-interactive runs).
trap '[ -t 0 ] && read -r -p "Press Enter to close..." _' EXIT

BIN="./build/hydra-cli"
if [ "${OS:-}" = "Windows_NT" ]; then
    BIN="./build/hydra-cli.exe"
fi

if [ ! -f "$BIN" ]; then
    echo "ERROR: $BIN not found. Run build.sh first." >&2
    exit 1
fi

"$BIN" "$@"
exit $?
