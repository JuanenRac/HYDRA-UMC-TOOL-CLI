#!/usr/bin/env bash
# HYDRA_UMC_SCRIPT_STANDARD_HEADER_BEGIN
# *****************************************************************************
# Project   : HYDRA-UMC-TOOL-CLI
# Script    : run.sh
# Purpose   : Runtime workflow for the project entry point.
# Author    : JuanenRac (Electro Hobby 3D)
# Email     : electrohobby3d@gmail.com
# Copyright : (C) 2026 JuanenRac
# License   : GPL-3.0 - see LICENSE
# *****************************************************************************
# HYDRA_UMC_SCRIPT_STANDARD_HEADER_END
# HYDRA_UMC_SCRIPT_STANDARD_BANNER_BEGIN
printf '\n*******************************************************************************\n'
printf '%s\n' "* HYDRA-UMC-TOOL-CLI - run.sh"
printf '%s\n' "* Mode      : RUN WORKFLOW"
printf '%s\n' "* Author    : JuanenRac (Electro Hobby 3D)"
printf '%s\n' "* Email     : electrohobby3d@gmail.com"
printf '%s\n' "* Copyright : (C) 2026 JuanenRac"
printf '%s\n' "* License   : GPL-3.0 - see LICENSE"
printf '%s\n' "* ------------------------------------------------------------------------- *"
printf '%s\n' "* 1. Resolve the runtime prerequisites declared by this script."
printf '%s\n' "* 2. Start the project entry point and forward user arguments unchanged."
printf '%s\n' "* 3. Preserve its result and keep an interactive terminal open."
printf '%s\n' "*******************************************************************************"
printf '\n'
# HYDRA_UMC_SCRIPT_STANDARD_BANNER_END
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
