@echo off
REM HYDRA_UMC_SCRIPT_STANDARD_HEADER_BEGIN
REM *****************************************************************************
REM Project   : HYDRA-UMC-TOOL-CLI
REM Script    : build.bat
REM Purpose   : Incremental project build, verification and packaging workflow.
REM Author    : JuanenRac (Electro Hobby 3D)
REM Email     : electrohobby3d@gmail.com
REM Copyright : (C) 2026 JuanenRac
REM License   : GPL-3.0 - see LICENSE
REM *****************************************************************************
REM HYDRA_UMC_SCRIPT_STANDARD_HEADER_END
REM HYDRA_UMC_SCRIPT_STANDARD_BANNER_BEGIN
echo.
echo *****************************************************************************
echo * HYDRA-UMC-TOOL-CLI - build.bat
echo * Mode      : INCREMENTAL BUILD
echo * Author    : JuanenRac (Electro Hobby 3D)
echo * Email     : electrohobby3d@gmail.com
echo * Copyright : (C) 2026 JuanenRac
echo * License   : GPL-3.0 - see LICENSE
echo * ------------------------------------------------------------------------- *
echo * 1. Read the project version (building never changes it; use the explicit bump command).
echo * 2. Run this project's declared build, verification and packaging commands.
echo * 3. Report the result and keep an interactive terminal open.
echo *****************************************************************************
echo.
REM HYDRA_UMC_SCRIPT_STANDARD_BANNER_END
REM Runs the real test suite, then compiles the Go
REM module in src/ into build\hydra-cli.exe. Run this before run.bat.
setlocal enabledelayedexpansion
rem Keep the window open for people, never in automation (CI / HYDRA_UMC_CI).
set "KEEP_OPEN=pause"
if defined CI set "KEEP_OPEN=ver >nul"
if "%HYDRA_UMC_CI%"=="1" set "KEEP_OPEN=ver >nul"
cd /d "%~dp0"
REM HYDRA_UMC_SCRIPT_STANDARD_VERSION_STEP
echo [1/5] Reading the project version (building never changes it)...
for /f "usebackq delims=" %%V in (`python -c "import json; print(json.load(open(r'%~dp0hydra-umc.project.json', encoding='utf-8'))['version'])"`) do set "HYDRA_UMC_VERSION_BUILT=%%V"
if not defined HYDRA_UMC_VERSION_BUILT set "HYDRA_UMC_VERSION_BUILT=unknown"
echo       Building v%HYDRA_UMC_VERSION_BUILT% (building never changes the version)
echo.
echo.

echo [2/5] Running the real test suite (go test)...
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

echo [3/5] Compiling Go module (src/cmd/hydra-cli)...
if not exist build mkdir build
pushd src
set "CGO_ENABLED=0"
go build -trimpath -ldflags "-s -w -X main.Version=%HYDRA_UMC_VERSION_BUILT%" -o ..\build\hydra-cli.exe .\cmd\hydra-cli
if errorlevel 1 (
    popd
    goto :error
)
popd
echo       Done. Binary: build\hydra-cli.exe
echo.

echo [4/5] Verifying the binary runs...
build\hydra-cli.exe version
if errorlevel 1 goto :error
echo.

REM C12: real cross-platform release packaging - GOOS/GOARCH env vars,
REM no separate toolchain needed. linux/arm64 is the CM5's own real
REM architecture; the rest cover an operator's own workstation.
echo [5/5] Packaging cross-platform release binaries...
set "RELEASE_DIR=build\release"
if exist "%RELEASE_DIR%" rmdir /s /q "%RELEASE_DIR%"
mkdir "%RELEASE_DIR%"

call :build_target linux amd64 hydra-cli
if errorlevel 1 goto :error
call :build_target linux arm64 hydra-cli
if errorlevel 1 goto :error
call :build_target windows amd64 hydra-cli.exe
if errorlevel 1 goto :error
call :build_target darwin amd64 hydra-cli
if errorlevel 1 goto :error
call :build_target darwin arm64 hydra-cli
if errorlevel 1 goto :error

where sha256sum >nul 2>nul
if not errorlevel 1 (
    pushd "%RELEASE_DIR%"
    (for %%D in (linux-amd64 linux-arm64 windows-amd64 darwin-amd64 darwin-arm64) do @for %%N in (hydra-cli hydra-cli.exe) do @if exist "./%%D/%%N" sha256sum "./%%D/%%N") > SHA256SUMS
    popd
) else (
    certutil -hashfile build\release\linux-amd64\hydra-cli SHA256 >nul 2>nul
    echo       (sha256sum not found - install it or use certutil per-file to checksum release\ manually)
)
echo       Done. Release artifacts: %RELEASE_DIR%\
echo.

echo ========================================
echo  Build complete. Run run.bat to execute the binary again.
echo  Cross-platform release artifacts: %RELEASE_DIR%\
echo ========================================
echo Built v%HYDRA_UMC_VERSION_BUILT%. Building never changes the version; to raise it run: python tools\hydra_project_tool.py bump
echo.
%KEEP_OPEN%
exit /b 0

:build_target
setlocal
set "GOOS=%~1"
set "GOARCH=%~2"
set "OUT_NAME=%~3"
set "OUT_DIR=%RELEASE_DIR%\%GOOS%-%GOARCH%"
if not exist "%OUT_DIR%" mkdir "%OUT_DIR%"
pushd src
set "CGO_ENABLED=0"
go build -trimpath -ldflags "-s -w -X main.Version=%HYDRA_UMC_VERSION_BUILT%" -o "..\%OUT_DIR%\%OUT_NAME%" .\cmd\hydra-cli
set "RESULT=%ERRORLEVEL%"
popd
if not "%RESULT%"=="0" (
    endlocal
    exit /b 1
)
echo       Built: %OUT_DIR%\%OUT_NAME%
endlocal
exit /b 0

:error
echo.
echo BUILD FAILED - see the output above.
%KEEP_OPEN%
exit /b 1
