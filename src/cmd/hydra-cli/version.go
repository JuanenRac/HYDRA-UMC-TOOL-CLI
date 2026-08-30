// =============================================================================
// HYDRA-UMC-TOOL-CLI - Version constant: cmd/hydra-cli/version.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0 - see LICENSE
// =============================================================================
// Single source of truth for this binary's version. Bumped by
// bump_version.py (odometer rule: PATCH+1, carrying into MINOR past 9)
// right before every real build - see build.sh/build.bat. Kept in its own
// file, separate from main.go, so the bump script has one small, stable
// regex target instead of hunting through command logic.
package main

// Version is the current release of HYDRA-UMC-TOOL-CLI.
const Version = "0.0.6"