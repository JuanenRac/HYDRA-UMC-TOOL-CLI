@echo off
REM =============================================================================
REM HYDRA-UMC-TOOL-CLI - Build and Compile Script
REM Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
REM GPL-3.0 - see LICENSE
REM =============================================================================
REM Bumps the version, runs the real test suite, then compiles the Go
REM module in src/ into build\hydra-cli.exe. Run this before run.bat.
setlocal enabledelayedexpansion
cd /d "%~dp0"

echo ========================================
echo  HYDRA-UMC-TOOL-CLI
echo  Build and Compile Script - bumps the version and compiles the CLI
echo  Author: JuanenRac (Electro Hobby 3D)
echo  E-mail: electrohobby3d@gmail.com
echo  License: GPL-3.0 - see LICENSE
echo ========================================
echo.

echo [1/4] Bumping version number (odometer bump, see bump_version.py)...
python bump_version.py
if errorlevel 1 ( echo NATIVE VERSION BUMP FAILED. & pause & exit /b 1 )
python "%~dp0bump_manifest_version.py" --sync
if errorlevel 1 ( echo VERSION SYNCHRONIZATION FAILED. & pause & exit /b 1 )
if errorlevel 1 goto :error
echo.

echo [2/4] Running the real test suite (go test)...
pushd src
go vet ./...
if errorlevel 1 (
    popd
    goto :error
)
go test ./...
if errorlevel 1 (
    popd
    goto :error
)
popd
echo       Done.
echo.

echo [3/4] Compiling Go module (src/cmd/hydra-cli)...
if not exist build mkdir build
pushd src
go build -o ..\build\hydra-cli.exe .\cmd\hydra-cli
if errorlevel 1 (
    popd
    goto :error
)
popd
echo       Done. Binary: build\hydra-cli.exe
echo.

echo [4/4] Verifying the binary runs...
build\hydra-cli.exe version
if errorlevel 1 goto :error
echo.

echo ========================================
echo  Build complete. Run run.bat to execute the binary again.
echo ========================================
pause
exit /b 0

:error
echo.
echo BUILD FAILED - see the output above.
pause
exit /b 1
