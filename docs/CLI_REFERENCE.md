# HYDRA-UMC-TOOL-CLI — CLI Reference

`hydra-cli` is a single Go binary (built with CGO disabled, so it has no shared-library dependencies) (`src/cmd/hydra-cli/`). Every
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
2. the `"server"` of a `--config PATH` file
3. the `HYDRA_CLI_SERVER` environment variable
4. the default, `http://localhost:3000`

The request timeout is the `--config` file's `"timeoutSec"`, then
`HYDRA_CLI_TIMEOUT_SEC`, then 5 seconds. A server value must be an absolute
`http://` or `https://` URL (a trailing `/` is dropped); a flag without a value
or an unknown argument is a usage error (exit code `2`), not silently ignored.

```bash
hydra-cli status --server http://192.168.1.50:3000
HYDRA_CLI_SERVER=http://192.168.1.50:3000 hydra-cli robots
```

## Authentication

`GET /api/hydra-info` (used by `status`) is public. `GET /api/settings` (used by
`robots` and by the second request of `doctor`) is only given to a logged-in
account: an administrator sees the full settings, any other role a copy with
the secrets redacted. The credential comes from the environment, never from a
flag, so it stays out of process listings and shell history:

| Variable | Meaning |
|---|---|
| `HYDRA_CLI_TOKEN` | a bearer token the server already issued |
| `HYDRA_CLI_USER` + `HYDRA_CLI_PASSWORD` | the CLI logs in with `POST /api/login` for that run |

The token wins when both are set. With neither, `robots` and `doctor` fail with
exit code `7` and name the variables to set; a refused login or an expired or
invalid token is also exit code `7`. Server responses are read up to 4 MiB. Logging in on every run is rate limited by the server (HTTP 429, reported as exit code `5`): for cron jobs and scripts set `HYDRA_CLI_TOKEN` instead. Over plain `http://` to another machine the CLI warns on stderr that the credential travels unencrypted.

```bash
HYDRA_CLI_USER=operator HYDRA_CLI_PASSWORD='...' hydra-cli robots --server http://192.168.1.50:3000
```

## Commands

### `version`

Prints the CLI's own name and version, then exits.

```
$ hydra-cli version
HYDRA-UMC-TOOL-CLI v0.1.5
```

### `help` / `-h` / `--help` / (no arguments)

Prints full command usage.

```
$ hydra-cli help
HYDRA-UMC-TOOL-CLI v0.1.5
Command-line interface for HYDRA-UMC/URTC fleet DevOps and automation.

USAGE:
    hydra-cli <command> [arguments]

COMMANDS:
    version              Print the CLI version and exit.
    status [--server URL] [--config PATH]
                          Query a HYDRA-UMC-SERVER instance's /api/hydra-info
                          endpoint and print its reported identity/version.
    robots [--server URL] [--config PATH] [--json]
                          Query a HYDRA-UMC-SERVER instance's /api/settings
                          endpoint and print its real controller/robot roster.
                          Both default to http://localhost:3000; override with --server, a
                          --config file's own "server" (see config validate/
                          apply below), or the HYDRA_CLI_SERVER environment
                          variable, in that precedence order.
	doctor [--server URL] [--config PATH] [--json]
	                      Read-only diagnostic: validates /api/hydra-info and
	                      /api/settings, then verifies their controller/robot
	                      counts agree. It never sends commands or probes hardware.
	                      --json prints a structured report (checkId/severity/
	                      status per check) instead of the DOCTOR=PASS line, for
	                      scripts that need to act on individual results.
    claims [--server URL] [--config PATH] [--json]
                          List who reserves which robot (GET /api/claims).
    claim <robot> [--ttl-ms N] [--force] [--reason TEXT]
    release <robot> [--reason TEXT]
                          Reserve a robot for this program, or release it. The
                          reservation belongs to your account AND to this
                          program's controller id (--controller-id ID or
                          HYDRA_CLI_CONTROLLER_ID; default hydra-cli-<host>).
                          --force is for administrators only.
    limits [--robot ID] [--json]
                          Read the safety limits the server holds (jog size,
                          absolute jog range, bed temperature, jog arming).
    jog-arm <robot> <on|off> [--ttl-ms N]
                          Arm or disarm the jog of one robot. Needed only when
                          the server requires it; ends by itself, with a stop
                          or when the robot is released.
    jog <robot> <axis> <amount> [--absolute]
                          Move one axis (x y z a b c). A step above the limit
                          of the robot's model is refused before it is sent.
    config validate --config PATH
                          Load and schema-validate a local config file.
    config apply --config PATH [--dry-run]
                          Preview (--dry-run) what a config apply would send.
                          Without --dry-run, exits ExitNotImplemented: no
                          live fleet-write endpoint exists yet.
    shell [--server URL] [--config PATH]
                          Interactive REPL - run any command above
                          repeatedly against the same server without
                          restarting the process. exit/quit or Ctrl-D
                          to leave.
    completion <bash|zsh>
                          Print a real completion script for the given
                          shell to stdout - eval "$(hydra-cli completion
                          bash)" (or source it from a completions
                          directory) to enable tab-completion.
    help                  Show this message.

    Every live command above also reads a --config file's own "timeoutSec"
    as its HTTP request timeout (default 5s), the same value config
    validate/apply already schema-validate.

    robots and doctor read /api/settings, which the server only gives to a
    logged-in account. Set HYDRA_CLI_TOKEN (a bearer token), or HYDRA_CLI_USER
    and HYDRA_CLI_PASSWORD (the CLI logs in for that run); they are read from
    the environment, never from a flag.

EXIT CODES:
    0  ok                     3  config error
    1  general error          4  network error (server unreachable)
    2  usage error            5  server error (bad status/response)
                              6  not implemented
                              7  authentication required or refused
                              8  refused by the server's rules (reserved,
                                 jog not armed, above a limit)

Report issues at: https://github.com/JuanenRac/HYDRA-UMC-TOOL-CLI
```

### `status [--server URL]`

Real `GET /api/hydra-info` against a live HYDRA-UMC-SERVER. Prints the
server's reported identity and version. An answer without an `appVersion` is
refused (exit code `5`): it is not a HYDRA-UMC-SERVER. A `status` line appears
only when the server publishes one.

```
$ hydra-cli status
server:      http://localhost:3000
product:     HYDRA-UMC TEST
hostname:    JUANEN
appVersion:  0.2.4
remoteApiVersion: 2
```

Unreachable server (real network error, exit code 4):

```
$ hydra-cli status --server http://127.0.0.1:1
hydra-cli status: could not reach http://127.0.0.1:1: Get "http://127.0.0.1:1/api/hydra-info": dial tcp 127.0.0.1:1: connectex: No connection could be made because the target machine actively refused it.
```

### `robots [--server URL]`

Real `GET /api/settings` against a live HYDRA-UMC-SERVER, with the credential
described under *Authentication*. Prints every controller's real robot roster —
name, online status, model, role.

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

### `claims`, `claim <robot>`, `release <robot>`

Who controls which robot (`GET /api/claims`), and reserving or releasing one
(`POST /api/robot/:id/claim` and `release`). A reservation belongs to the
account **and** to the controller id of this program, sent in the body and in
the `X-Controller-Id` header of every request. The id comes from
`--controller-id ID`, then `HYDRA_CLI_CONTROLLER_ID`, then `hydra-cli-<host>`
(1 to 64 characters: letters, digits, `. _ : -`); inside `shell` it stays the
same for the whole session.

```
hydra-cli claims [--json]
hydra-cli claim 3 [--ttl-ms 60000] [--reason "bench"] [--force]
hydra-cli release 3 [--reason "done"]
```

`--force` (administrators only) takes over a reservation held by someone else.
The server's refusals are told apart and exit with code `8`: the robot is
reserved by another account (`other-account`), by your account from another
controller (`other-controller`), the reservation names a controller and the
request named none (`controller-id-required`), or the server requires the id
to reserve. An expired login is exit code `7`.

### `limits [--robot ID] [--json]`

Reads `GET /api/safety-limits`: the largest relative jog, the largest absolute
jog, the highest bed target, whether the jog has to be armed, how long an
arming lasts and whether a reservation has to name its controller. With
`--robot ID` it also prints the values that apply to that robot's model and how
long its jog stays armed.

### `jog-arm <robot> <on|off> [--ttl-ms N]` and `jog <robot> <axis> <amount> [--absolute]`

`jog-arm` arms or disarms the jog of one robot. It is only needed when the
administrator turned on `jog.armRequired`; the arming ends by itself, with a
stop or when the robot is released. `jog` moves one axis (`x y z a b c`). Before
sending, the CLI reads the limits of the robot's model: a step above the
relative limit, or a target above the absolute limit, is refused with exit code
`8` and nothing is sent; if the limits cannot be read nothing is sent either.
If the server refuses with `jog-not-armed`, the message names the `jog-arm`
command. Warnings returned by the server are printed (`Warning (jog): ...`) and
the command exits with code `8`, since the jog was not applied.

### `doctor [--server URL]`

Performs a read-only contract diagnosis against the two Server endpoints used by
this CLI: the public `GET /api/hydra-info` and `GET /api/settings`, which needs
the credential described under *Authentication*.
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
Server is a network error (exit code `4`); a missing or refused credential is
exit code `7`. Older Servers which do not publish
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
DOCTOR=PASS server=http://192.168.1.50:3000 appVersion=0.2.4 schema=1.0 remoteApiVersion=2 controllers=1 robots=2 countCrossCheck=pass
hydra-cli> exit
```

## Read-only views of the server and the plan simulation

These four commands only read or simulate: they never change a robot, a reservation or a setting. Each answer is checked against the closed shape of its contract before a byte is printed. An answer that is not an object, lacks a field the contract requires, has a field of the wrong type, holds a value outside the closed lists of the contract, or whose summary does not add up to its list, is an error (exit code 5) and prints nothing; a field the contract does not know is never passed on. Every text that came from a server is cleaned of control characters and of anything that looks like a credential. They use the same HTTP client, the same options (`--server`, `--config`, `--timeout`) and the same login (`HYDRA_CLI_TOKEN`, a saved session or `HYDRA_CLI_USER`/`HYDRA_CLI_PASSWORD`) as the other commands, and the same exit codes (7 without a login or with a refused one, 4 when the server cannot be reached).

### `connections [clients|events] [--kind KIND] [--user NAME] [--active] [--action ACTION] [--limit N] [--before N] [--json]`

`GET /api/connections/clients` (the default) lists the clients that connect to the server; `GET /api/connections/events` lists the history of their sign-ins. An administrator sees every client; any other account sees only its own entries, and the address and user agent only when the administrator turned `connections.recordAddress` on. `--kind` is one of `studio suite android ios dsi watch voice other`; `--action` one of `first_seen login login_failed second_factor_failed logout session_ended session_expired`; `--limit` and `--before` are whole numbers of up to nine digits. Filters that do not fit the chosen list are a usage error (exit 2) and nothing is sent. With `--json`: `{"schema_version": "1.0", "command": "connections clients", "ok": true, "clients": [...]}` or `"command": "connections events"` with `"events": [...]`; each item has the fields of the `ConnectionClient` or `ConnectionEvent` contract of the SDK.

### `liveness [--json]`

`GET /api/robots/liveness`: for each robot of the roster, `live`, `stale` (no news for longer than the limit the server holds) or `unknown` (no news since the server started), with the age of the last news and its source (`state` or `heartbeat`). With `--json`: the envelope plus `stale_after_s` and `robots` (`controller_id`, `robot_id`, `state` and, unless unknown, `last_update_utc`, `age_s`, `source`). A robot that is `unknown` and also reports news, or one that is `live` or `stale` and reports none, is a malformed answer.

### `ecosystem-health [--json]`

`GET /api/ecosystem/health`: the state, the reason and the version drift of each service of the ecosystem, the certificate of the server and the warnings. Needs an operator or an administrator account, and the server must have `ecosystemHealth.enabled` switched on: otherwise it answers 404, and the command says the feature is missing or switched off (exit 5). With `--json`: the envelope plus `generated_at_utc`, `available`, `summary` (`total ok degraded down unknown drift`), `services`, `certificate` (`null` when there is none) and `warnings`; fields that are unknown to the server stay `null`, and the port, health path and unit of a service appear only for an administrator.

### `plan-simulate --plan FILE|- [--planner URL] [--json]`

Sends a plan (one JSON object with a `steps` list and optional `constraints`) to `POST /plan/simulate` of the semantic planner, a service of its own that does not use the login of the server: the credential of the server is never sent to it. The planner is `--planner URL`, then `HYDRA_CLI_PLANNER`, then `http://localhost:8109`; its switch `plan_simulation.enabled` must be on (otherwise it answers 404, and the command says so). `-` reads the plan from the standard input. Nothing is executed, queued or released, by the planner or by this command.

| Outcome | Exit code |
|---|---|
| the plan is accepted | 0 |
| the simulation rejects the plan (every broken rule is printed) | 8 |
| the planner cannot understand the plan (HTTP 400), or the plan file is not one JSON object | 2 |
| the planner does not offer the route (404), answers with another error, or its answer does not match the contract | 5 |
| the planner cannot be reached | 4 |

A verdict is checked before it is printed: it must be `accepted` or `rejected`, say `executable: false` and `executed: false`, list violations only when rejected (each with a `rule` and a `message`) and carry a timeline whose steps do not end before they start. With `--json`: the envelope (`ok` is `true` only for an accepted plan) plus `verdict`, `executable`, `executed`, `violations`, `timeline`, `makespan_ms` and `simulated_clock_ms`. An `accepted` verdict means the plan breaks none of the declared rules, not that it is safe to run.

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | ok |
| `1` | general error (unclassified) |
| `2` | usage error — unknown command or missing/malformed arguments |
| `3` | config error — a local config file failed validation |
| `4` | network error — the target server could not be reached at all |
| `5` | server error — the server responded, but with a non-2xx status or a response this CLI couldn't parse or that is larger than 4 MiB |
| `6` | not implemented — a real, honest "this can't run live yet" outcome |
| `7` | authentication required or refused |

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

## Sessions, credentials and the newer commands

- `login [--user NAME] [--password-stdin]`, `logout`: see the CHANGELOG; the credential is looked for in this order: `HYDRA_CLI_TOKEN`, the saved session of the server, `HYDRA_CLI_USER` + `HYDRA_CLI_PASSWORD`. `login` signs in once (user from `--user` or `HYDRA_CLI_USER`, password from `HYDRA_CLI_PASSWORD` or `--password-stdin`, never from an option) and saves the token and refresh token in a file only the user can read: `HYDRA_CLI_TOKEN_FILE`, `tokenFile` of a config file, or `hydra-cli/session.json` in the user configuration directory. Later commands use it and renew it with `POST /api/refresh` when the server says the token expired; `logout` revokes the refresh token and removes the file. With only `HYDRA_CLI_USER` and `HYDRA_CLI_PASSWORD` the CLI logs in at most once per process (a `shell` shares one login) and revokes the refresh token when it exits.
- Credentials are refused over plain http to another machine unless `HYDRA_CLI_ALLOW_INSECURE=1` (or `allowInsecure` in the config file); `HYDRA_CLI_CA_FILE` (or `caFile`) trusts a private CA. The config file also accepts `tokenFile`.
- `--timeout SECONDS` (1 to 3600) on every live command.
- `scan <network|address>... [--ports 3000,3443] [--scheme http|https] [--concurrency N] [--max-hosts N] [--probe-timeout-ms N] [--allow-public-range] [--json]`: private networks only unless allowed, at most 1024 addresses by default.
- `fleet doctor --servers FILE [--concurrency N] [--json]`: exit 0 when all pass, otherwise the code the failures share, or 1.
- `stop <robot>`, `stop-all --yes [--json]`: software stops, not the emergency stop of the cell.
- `metrics [--all] [--json]`, `manifest validate [PATH...] [--workspace DIR] [--schema FILE] [--json]`, `status --json`, `completion <bash|zsh|fish|powershell>`.

## JSON output (`--json`)

Every command that prints data also prints it as JSON with `--json`. The text output is the default and does not change. A failure is never printed as JSON: it goes to the standard error as text and sets the exit code listed above, with or without `--json`. No JSON output ever carries a token, a refresh token or a password.

There are two shapes.

**Commands that read** keep the shape of what they read:

| Command | Output with `--json` |
|---|---|
| `status` | one object: `server`, `product`, `hostname`, `appVersion`, `remoteApiVersion`, `status` |
| `robots` | an array (empty when there are no robots) of `controller`, `robot`, `online`, `model`, `role` |
| `claims` | an array of the reservations the server reports |
| `robot`, `limits`, `stop-all` | the answer of the server, indented |
| `doctor` | one report: `server`, `ok`, `controllers`, `robots`, `checks` (`checkId`, `severity`, `status`) |
| `metrics`, `scan`, `fleet doctor`, `manifest validate`, `audit` | the samples, servers found, reports, verdicts or lines they list |

**Commands that act, and the commands added with the JSON mode,** print one object with a fixed envelope: `schema_version` (`"1.0"`), `command`, `ok` and the fields of the command. The keys are sorted and the fields of a command do not change within a `schema_version`; a new field is a new `schema_version`.

| Command | Fields besides the envelope |
|---|---|
| `version` | `name`, `version` |
| `stop` | `robot`, `revision` (`null` when the server sends none) |
| `claim`, `release` | `robot`, `controller_id`, `revision` |
| `jog-arm` | `robot`, `armed`, `expires_at_ms` (`null` when disarmed) |
| `jog` | `robot`, `axis`, `amount`, `absolute`, `revision`; when the server did not apply the jog, `ok` is `false`, `warnings` (`field`, `code`, `message`) is listed and the exit code is 8 |
| `table` | `robot`, `x`, `y`, `rz` (`null` when not moved), `revision` |
| `login` | `server`, `role`, `session_file`, `refresh_token_saved` |
| `logout` | `server`, `session_found`, `revoked_on_server`, `session_file` (`null` when there was no session) |
| `config validate` | `path`, `server`, `timeout_sec`, `ca_file`, `allow_insecure`, `token_file` (`null` when not set) |
| `config apply --dry-run` | `dry_run`, `server`, `timeout_sec`, `contacted` (always `false`) |
| `connections clients`, `connections events` | `clients` or `events` (see above) |
| `liveness`, `ecosystem-health`, `plan-simulate` | see above |

`help`, `shell` and `completion` have no data to print and take no `--json`.
