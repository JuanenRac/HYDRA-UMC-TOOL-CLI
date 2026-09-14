# =============================================================================
# HYDRA-UMC-TOOL-CLI - Read-only Server diagnostic: docs/DOCTOR.md
# Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
# GPL-3.0 - see LICENSE
# =============================================================================

# HYDRA-UMC-TOOL-CLI Doctor

`hydra-cli doctor` is a safe, read-only compatibility check for a running
HYDRA-UMC-SERVER. It is intended for local development, CI fixtures and early
deployment checks before any hardware operation is attempted.

## What it verifies

1. `GET /api/hydra-info` responds with HTTP 200 and valid JSON containing an
   `appVersion`.
2. `GET /api/settings` responds with HTTP 200 and valid JSON.
3. When `/api/hydra-info` publishes `controllerCount` and `robotCount`, each
   total matches the actual controllers and nested robots in `/api/settings`.

The command returns `0` only after those applicable checks pass. It returns
`4` when the Server cannot be reached and `5` for bad HTTP responses, invalid
JSON, missing `appVersion`, or a published count mismatch.

## What it deliberately does not verify

Doctor does not issue a command or perform a physical diagnosis. It does not
connect to CAN, cameras, Hailo, motion controllers, sensors, actuators, or a
CM5. A `DOCTOR=PASS` result proves the two Server read contracts are coherent;
it must not be interpreted as a safety or hardware certification.

## Usage

```bash
# Default Server target: http://localhost:3000
./run.sh doctor

# Explicit target
./run.sh doctor --server http://192.168.1.50:3000

# Or set the shared target once for this shell
HYDRA_CLI_SERVER=http://192.168.1.50:3000 ./run.sh doctor
```

On Windows, replace `./run.sh` with `run.bat`.

## Output contract

```text
DOCTOR=PASS server=http://localhost:3000 appVersion=0.2.4 schema=1.0 remoteApiVersion=2 controllers=1 robots=8 countCrossCheck=pass
```

`countCrossCheck=pass` means the Server explicitly reported counts and they
matched the settings roster. `countCrossCheck=not-reported` means the Server
is an older compatible endpoint that did not expose both count fields; the
CLI reports that absence honestly rather than claiming a count comparison.

## `--json`: structured output for scripts

```bash
./run.sh doctor --json
```

Instead of the single `DOCTOR=PASS ...` line, `--json` prints a report with
one entry per real check:

```json
{
  "server": "http://localhost:3000",
  "ok": true,
  "appVersion": "0.2.4",
  "schemaVersion": "1.0",
  "remoteApiVersion": 2,
  "controllers": 1,
  "robots": 8,
  "checks": [
    { "checkId": "hydra-info-reachable", "severity": "critical", "status": "pass" },
    { "checkId": "hydra-info-has-app-version", "severity": "critical", "status": "pass" },
    { "checkId": "settings-reachable", "severity": "critical", "status": "pass" },
    { "checkId": "controller-count-cross-check", "severity": "warning", "status": "pass" },
    { "checkId": "robot-count-cross-check", "severity": "warning", "status": "pass" }
  ]
}
```

`status` is `"pass"`, `"fail"`, or (for the two count cross-checks against an
older Server that never published both count fields) `"not-reported"`. A
`"fail"` entry carries a `message` describing what went wrong. `severity`
distinguishes a `"critical"` check (doctor could not complete at all - an
unreachable Server, a malformed response) from a `"warning"` one (the same
best-effort count comparison the plain-text `countCrossCheck` already
tolerates being absent). The report still appears on a failing run, listing
whichever checks ran before the failure with `"ok": false` - `--json` only
changes what stdout carries; the command's exit code and pass/fail verdict
are identical to plain-text mode either way.
