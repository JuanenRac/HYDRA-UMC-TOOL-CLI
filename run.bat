@echo off
REM =============================================================================
REM HYDRA-UMC-TOOL-CLI - Run Script
REM Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
REM GPL-3.0 - see LICENSE
REM =============================================================================
REM Runs the compiled binary. Run build.bat first. Forwards any arguments
REM through to hydra-cli.exe (e.g. "run.bat status --server http://host:3000").
cd /d "%~dp0"

if not exist build\hydra-cli.exe (
    echo ERROR: build\hydra-cli.exe not found. Run build.bat first.
    exit /b 1
)

build\hydra-cli.exe %*
pause
