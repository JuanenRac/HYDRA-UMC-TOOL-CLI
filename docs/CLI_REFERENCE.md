# HYDRA-UMC-TOOL-CLI — CLI Reference

`hydra-cli` is a single static Go binary (`src/cmd/hydra-cli/`). Every
command below was captured from a real, built binary — the output shown
is real, not illustrative.

## Usage

```
hydra-cli <command> [arguments]
```

Running `hydra-cli` with no arguments, `help`, or `-h`/`--help` prints
the same usage summary shown by `help` below.

## Server resolution

`status`, `robots`, `doctor`, and `config apply` (once live writes exist) all
target a HYDRA-UMC-SERVER instance. The target is resolved in this order:

1. `--server URL` on the command line
2. the `HYDRA_CLI_SERVER` environment variable
3. the default, `http://localhost:3000`

```bash
hydra-cli status --server http://192.168.1.50:3000
HYDRA_CLI_SERVER=http://192.168.1.50:3000 hydra-cli robots
```

## Commands

### `version`

Prints the CLI's own name and version, then exits.

```
$ hydra-cli version
HYDRA-UMC-TOOL-CLI v0.0.5
```

### `help` / `-h` / `--help` / (no arguments)

Prints full command usage.

```
$ hydra-cli help
HYDRA-UMC-TOOL-CLI v0.0.5
Command-line interface for HYDRA-UMC/URTC fleet DevOps and automation.

USAGE:
    hydra-cli <command> [arguments]

COMMANDS:
    version              Print the CLI version and exit.
    status [--server URL] Query a HYDRA-UMC-SERVER instance's /api/hydra-info
                          endpoint and print its reported identity/version.
    robots [--server URL] Query a HYDRA-UMC-SERVER instance's /api/settings
                          endpoint and print its real controller/robot roster.
                          Both default to http://localhost:3000; override with --server or the
                          HYDRA_CLI_SERVER environment variable.
    config validate --config PATH
                          Load and schema-validate a local config file.
    config apply --config PATH [--dry-run]
                          Preview (--dry-run) what a config apply would send.
                          Without --dry-run, exits ExitNotImplemented: no
                          live fleet-write endpoint exists yet.
    help                  Show this message.

EXIT CODES:
    0  ok                     3  config error
    1  general error          4  network error (server unreachable)
    2  usage error            5  server error (bad status/response)
                              6  not implemented

Report issues at: https://github.com/JuanenRac/HYDRA-UMC-TOOL-CLI
```

### `status [--server URL]`

Real `GET /api/hydra-info` against a live HYDRA-UMC-SERVER. Prints the
server's reported identity and version. Current Servers do not publish a
`status` field; the CLI says `not reported` honestly rather than printing an
empty value.

```
$ hydra-cli status
server:      http://localhost:3000
product:     HYDRA-UMC TEST
hostname:    JUANEN
appVersion:  0.2.4
remoteApiVersion: 2
status:      not reported
```

Unreachable server (real network error, exit code 4):

```
$ hydra-cli status --server http://127.0.0.1:1
hydra-cli status: could not reach http://127.0.0.1:1: Get "http://127.0.0.1:1/api/hydra-info": dial tcp 127.0.0.1:1: connectex: No connection could be made because the target machine actively refused it.
```

### `robots [--server URL]`

Real `GET /api/settings` against a live HYDRA-UMC-SERVER. Prints every
controller's real robot roster — name, online status, model, role.

```
$ hydra-cli robots
CONTROLLER     ROBOT                ONLINE   MODEL                    ROLE
HYDRA-UMC Master Robot A1             yes      Parol6 (6-DOF)           Pnp
HYDRA-UMC Master Robot A2             yes      Faze4 (6-DOF)            CNC
HYDRA-UMC Master Robot A3             yes      Parol6 (6-DOF)           Inspection
HYDRA-UMC Master Robot A4             no       Faze4 (6-DOF)            Idle
HYDRA-UMC Master Robot A5             no       Parol6 (6-DOF)           Idle
HYDRA-UMC Master Robot A6             no       Faze4 (6-DOF)            Idle
HYDRA-UMC Master Robot A7             no       Parol6 (6-DOF)           Idle
HYDRA-UMC Master Robot A8             no       Faze4 (6-DOF)            Idle
```

If the server reports zero robots, `robots` prints
`no robots reported by <server>` instead of an empty table.

### `doctor [--server URL]`

Performs a read-only contract diagnosis against the same two public Server
endpoints used by this CLI: `GET /api/hydra-info` and `GET /api/settings`.
It confirms that both responses are valid and, when the Server reports fleet
counts, that its published controller and robot totals match the roster in
`/api/settings`.

```text
$ hydra-cli doctor
DOCTOR=PASS server=http://localhost:3000 appVersion=0.2.4 schema=1.0 remoteApiVersion=2 controllers=1 robots=8 countCrossCheck=pass
```

This is not a physical health test: it never sends a command and does not
probe CAN, cameras, sensors, motion, or safety hardware. A mismatch, invalid
JSON, or non-200 response is a server error (exit code `5`); an unreachable
Server is a network error (exit code `4`). Older Servers which do not publish
the two count fields remain supported but report `countCrossCheck=not-reported`
instead of a false pass.

### `config validate --config PATH`

Loads and schema-validates a local config file (`server`: an absolute
URL; `timeoutSec`: a positive integer). No network access.

```json
// hydra-cli.json
{"server": "http://localhost:3000", "timeoutSec": 5}
```

```
$ hydra-cli config validate --config hydra-cli.json
config hydra-cli.json is valid: server=http://localhost:3000 timeoutSec=5
```

Invalid config (exit code 3):

```
$ hydra-cli config validate --config broken.json
hydra-cli config: config broken.json: "server" must be a valid absolute URL (e.g. "http://host:port"), got "not-a-url"
```

### `config apply --config PATH [--dry-run]`

`--dry-run` loads and validates the config, then prints exactly what a
live apply would send — no device or server is contacted:

```
$ hydra-cli config apply --config hydra-cli.json --dry-run
DRY RUN: would apply config to http://localhost:3000
  server:     http://localhost:3000
  timeoutSec: 5
no real device or server was contacted
```

Without `--dry-run`, this honestly refuses (exit code 6) rather than
pretending to push config to a write endpoint that does not exist yet on
HYDRA-UMC-SERVER:

```
$ hydra-cli config apply --config hydra-cli.json
hydra-cli config: live config apply needs a real fleet-write endpoint on HYDRA-UMC-SERVER, which does not exist yet - rerun with --dry-run
```

### `shell [--server URL]`

An interactive REPL: run any command above repeatedly against the same
server without restarting the process. Dispatches every line through the
exact same command table one-shot invocations use, so shell and one-shot
behavior never drift apart. `--server`, if given, sets `HYDRA_CLI_SERVER`
for the rest of the session — the same environment variable a one-shot
invocation already honors — so lines typed afterward don't need their own
`--server` unless they want to target something else for that one line.

Lines are tokenized the way an operator expects: `'...'`/`"..."` group
their own contents (including embedded spaces) into a single argument, real
for a `--config` path with a space in it. `exit`, `quit`, or Ctrl-D (EOF)
leaves the shell; a blank line is ignored.

```
$ hydra-cli shell --server http://192.168.1.50:3000
HYDRA-UMC-TOOL-CLI vX.Y.Z interactive shell - target http://192.168.1.50:3000
Type a command (version, status, robots, doctor, config ...), or exit/quit to leave.
hydra-cli> status
server:      http://192.168.1.50:3000
appVersion:  0.2.4
hydra-cli> doctor
DOCTOR: OK - controllerCount and robotCount agree with the real roster (1/2)
hydra-cli> exit
```

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | ok |
| `1` | general error (unclassified) |
| `2` | usage error — unknown command or missing/malformed arguments |
| `3` | config error — a local config file failed validation |
| `4` | network error — the target server could not be reached at all |
| `5` | server error — the server responded, but with a non-2xx status or a response this CLI couldn't parse |
| `6` | not implemented — a real, honest "this can't run live yet" outcome |

```
$ hydra-cli bogus
hydra-cli: unknown command "bogus"
...
$ echo $?
2
```

## Planned, not yet implemented

`deploy`, `flash-all`, and `audit` are described in the project README's
roadmap but are not built yet — each needs a fleet write/CAN-OTA/write
endpoint on HYDRA-UMC-SERVER that doesn't exist. `config apply` without
`--dry-run` (above) is the current, honest placeholder for the same gap.
