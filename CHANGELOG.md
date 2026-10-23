# Changelog

All notable changes to this project are documented here.

## [Unreleased]

- **Conditional writes.** `claim`, `release`, `jog` and the new `table` take `--if-match REVISION` (a revision printed by `robot` or by an earlier write) or `--if-match current` (the CLI reads the robot first and sends the revision it has now). The request carries it as `If-Match`; when the robot changed in between, the server answers 412, nothing is applied and the CLI exits with code 8 saying which revision the robot is at now. Every successful write prints `Revision: ...` so a script can chain the next one. Without the option nothing changes.
- **`hydra-cli robot <robot>`** reads one robot (`GET /api/robot/:id`) with its revision and cell (`--json` prints the answer).
- **`hydra-cli table <robot> <x> <y> [--rz N] [--relative]`** moves the XY table in one atomic request. The move is checked as a whole against the limits the server holds for the model before anything is sent, an older server that does not know the command is told apart from a refusal, and the position the server applied is printed.
- **`hydra-cli audit [--limit N] [--type T] [--robot ID] [--verify] [--export FILE] [--json]`** reads the audit record (administrators only), newest line first, and checks the chain of SHA-256 hashes of the lines it was given (a changed line, a replaced line whose successor still names the old one; a filtered window only shows gaps). A broken chain exits with code 8. `--verify` also has the server check the whole record, `--export` saves the lines as stored, and an off or locked-off record, a non-administrator account and an older server each get their own message.
- Tests for the three commands and for the `--if-match` option (the quoted header, `current`, validation, 412), with a fake server.

- **Shared secret redaction**: `redact` is a Go port of the common module and passes the same 35 shared vectors (`redact/testdata/vectors.json`); `printable`, through which every text taken from a server is printed, now hides secret shapes as well as control characters, so a token or password in a server answer is never echoed. Go's regexp has no look-behind, look-ahead or back-references, so those are done by hand; a password inside a URL is only recognised up to 1000 characters.
- **Reservations, safety limits and jog, the same contract as the other clients.** New commands: `claims` (who reserves which robot, `--json`), `claim <robot>` and `release <robot>`, `limits [--robot ID]` (the limits the server holds), `jog-arm <robot> <on|off>` and `jog <robot> <axis> <amount> [--absolute]`.
- Every reservation, release, arming and jog request carries a controller id for the session (`--controller-id`, `HYDRA_CLI_CONTROLLER_ID`, default `hydra-cli-<host>`), so a reservation belongs to this program and not to the whole account.
- The refusals of the server are told apart in plain messages: reserved by another account, by your account on another controller, controller id required, and jog not armed (with the command to arm it). They exit with the new code `8` (refused by the server's rules).
- A jog above the relative or absolute limit of the robot's model is refused before it is sent, nothing is sent when the limits cannot be read, and the warnings of the server are printed and make the command fail. Tab completion and the interactive shell know the new commands.
- Building no longer changes the version; raising it is the explicit `bump` command (`python tools/hydra_project_tool.py bump`). The build scripts only report the version they built, and CI fails if a build script runs a bump.

## [0.1.7] - An aligned robots table, JSON output and exit codes in scripts

- The banner follows the animated style of the ecosystem banners, with the name, the kind and the technologies of the project.
- **`robots` prints an aligned table** whatever the length of the names, and `robots --json` writes the roster as a JSON array (`[]` when the server reports no robots), like `doctor --json`.
- **The interactive shell exits with the code of its last command,** so `echo ... | hydra-cli shell` can be used in scripts, and a failure to read the input is reported instead of ending as a clean exit.
- **`build.bat` writes `SHA256SUMS` with relative paths** (`./linux-amd64/hydra-cli`), the same as `build.sh`, so the file verifies from the release folder.
- **The READMEs in the eight languages list `completion`** among the implemented commands and the architecture diagram shows the config file and the shell completion next to the server.
- Added `tools/hydra_project_tool.py`: the same `validate`, `test`, `version`, `status` and `bump` commands in every repository. The checks run with `HYDRA_UMC_CI=1` and fail if the version changes; only `bump` changes it.

## [0.1.6]

- Maturity is functional: nothing here has been verified on real hardware yet.
- The READMEs say, in the eight languages, what is planned and what has only been tested against fakes, emulators or fixtures, and carry a short safety and security status note.

## [0.1.5] - Server text is printed safely, and config files must name an http(s) server

- **Output safety:** names, versions, products, hostnames and error bodies that come from the server are printed with control characters (newlines, escape sequences) replaced by `?`, so a hostile or broken server can no longer forge extra table rows, status lines or a `DOCTOR=PASS` line, or drive the operator's terminal with escape codes. `--json` output was already escaped.
- **Config files:** `config validate` and `config apply --dry-run` now require an `http://` or `https://` server, like every live command, instead of calling an `ftp://` or `ws://` server valid and letting the next command refuse it.
- 19 new tests (105 in all, 94.8 % statement coverage, up from 86.3 %): the command dispatcher and its exit codes, hostile server text, hostile config files, server and timeout parsing, credential edge cases (blank and padded tokens, unusable login answers) and the exact response-size limit.

## [0.1.4] - Stricter arguments, safer logins, accurate reference

- **Arguments:** `status` and `robots` reject unknown arguments (a mistyped `--sever` no longer silently queries another server), and `--server`/`--config` without a value are usage errors (exit code 2). A server value must be an absolute http(s) URL, a trailing `/` is dropped for every command, and a bad value is a usage error instead of a network error.
- **`status`:** refuses an answer without `appVersion` (a foreign JSON service no longer "passes") and no longer prints the `status: not reported` line the Server never sends.
- **`doctor`:** checks `schema_version` of `/api/hydra-info` (new `hydra-info-schema-version` check) and always emits the `controllers` and `robots` counters in `--json`.
- **Logins:** an HTTP 429 on `POST /api/login` explains the rate limit and points to `HYDRA_CLI_TOKEN`; a warning goes to stderr when a credential would travel over plain `http://` to another machine.
- **Responses over 4 MiB** are now reported as a server error (exit code 5) instead of "server unreachable".
- **Build:** the native binary is built with `CGO_ENABLED=0` (truly static), the step numbering of `build.sh` is fixed, and `run.bat` keeps the CLI exit code and only pauses for an interactive run without arguments.
- **Repo hygiene:** the stale compiled binary that had been committed is untracked and `.gitignore` covers it.
- **Docs:** `docs/CLI_REFERENCE.md` regenerated from the real binary (version, help, server resolution, exit code 7, `doctor` output), the READMEs require Go 1.24 as `go.mod` does, and the manifest lists the real technologies and test count.

## [0.1.3] - Credentials for the authenticated reads, exit code 7, bounded responses

- **`robots` and `doctor` worked against no real Server:** HYDRA-UMC-SERVER only answers `GET /api/settings` to a logged-in account, but the CLI sent it with no credential, so against a current Server both commands ended in `HTTP 401` (exit code 5) while the docs called the read "unauthenticated". The CLI now takes a credential from the environment - `HYDRA_CLI_TOKEN`, or `HYDRA_CLI_USER` and `HYDRA_CLI_PASSWORD` (it logs in with `POST /api/login` for that run; the token wins) - never from a flag, so it stays out of process listings and shell history. `status` and the first request of `doctor` use the public `/api/hydra-info` and never send it. Checked end to end against a real HYDRA-UMC-SERVER process: no credential, wrong password, login and token.
- **New exit code 7:** authentication required or refused (no credential given, a refused login, an expired or invalid token), kept apart from `5` so a script can tell "set a credential" from "the server is broken". Documented in `help`, the README and `docs/CLI_REFERENCE.md`.
- **Bounded responses:** every response is read up to 4 MiB and a larger one is refused (exit code 4), instead of reading whatever an endpoint sends into memory.
- **Tests:** 11 new tests cover a guarded fake server (no credential, wrong token, token, login, wrong password, user without password, token winning over login), that the public endpoint never receives the credential, doctor with a credential, and the size cap (83 tests).
- **Docs:** the README no longer says the settings read is unauthenticated, states that the commands were run against a real Server, and a README in Portuguese (`README_por.md`) joins the other translations.

## [0.1.2] - The JSON contract and unattended use are pinned by tests

- `doctor --json` now has a test pinning its key set (top-level and per check) and asserting
  no terminal colour codes, so automation cannot be broken by a rename unnoticed. A second
  test runs it with nothing on stdin and checks the output never looks like a prompt: the CLI
  handles no credentials and never asks for any.

## [0.1.1] - Real shell completion

New `hydra-cli completion <bash|zsh>` prints a real completion script
for the given shell - the subcommand list it suggests is kept as one
small, explicit slice mirroring `run()`'s own real dispatch table in
main.go, not reflection or a guess. 5 new tests.

## [0.1.0]

- **`hydra-cli doctor --json`** - the diagnostic previously only ever printed
  a single `DOCTOR=PASS ...` prose line, and printed nothing at all to
  stdout on failure. `--json` now emits a structured report instead: a
  `checkId`/`severity`/`status` entry per real check (`hydra-info-reachable`,
  `hydra-info-has-app-version`, `settings-reachable`,
  `controller-count-cross-check`, `robot-count-cross-check`), plus an overall
  `ok` flag and the same facts (`appVersion`, `controllers`, `robots`, ...)
  the plain-text line already reported. A failure still returns the exact
  same error and exit code as before - `--json` only changes what stdout
  carries, never doctor's pass/fail verdict. Plain-text `doctor` (no
  `--json`) is byte-for-byte unchanged.

## [0.0.9] - real cross-platform release packaging

`build.sh`/`build.bat` only ever produced ONE native binary, for running
on the machine doing the build - nothing here ever cross-compiled a real
release for the CM5 itself or an operator's other platforms.

- `build.sh`/`build.bat` now also cross-compile, via Go's own real
  `GOOS`/`GOARCH` (no separate toolchain needed): `linux/amd64`,
  `linux/arm64` (the CM5's own real architecture),
  `windows/amd64`, `darwin/amd64`, `darwin/arm64` - into
  `build/release/<os>-<arch>/hydra-cli(.exe)`, plus a real `SHA256SUMS`
  checksum file over every artifact (`sha256sum` on Linux/macOS/Git
  Bash, falling back to `certutil` on plain Windows `cmd`).
- The existing native build/test/run step is unchanged - this is purely
  additive packaging after it, not a replacement.

## [0.0.8] - `--config` now really drives every live command, not just validation

- Found on code review: `config
  validate`/`config apply --dry-run` already schema-validated a real
  local config file's `server`/`timeoutSec` (`config.go`), but no live
  command ever consulted it - `status`/`robots`/`doctor`/`shell` only
  ever looked at `--server`/`HYDRA_CLI_SERVER`/the compiled-in default,
  with a hardcoded 5s timeout in every one of them.
- **`server.go`** - `resolveServer()` replaced by `resolveTarget()`,
  which resolves BOTH the target server AND the request timeout.
  Precedence for the server: `--server` flag > `--config` file's own
  `"server"` > `HYDRA_CLI_SERVER` env var > the compiled-in default.
  Precedence for the timeout: `--config` file's own `"timeoutSec"` >
  `HYDRA_CLI_TIMEOUT_SEC` env var > a new `defaultRequestTimeout` (5s,
  the same value every command already hardcoded). A malformed/invalid
  `--config` file surfaces the exact same real `ExitConfigError`
  `config validate` already produces - never silently ignored.
- **`main.go`/`robots.go`/`doctor.go`** - `cmdStatus`/`cmdRobots`/
  `cmdDoctor` now call `resolveTarget()` instead of `resolveServer()`
  and build their `http.Client` with the resolved timeout instead of a
  hardcoded `5 * time.Second`. Help text documents the new `--config`
  flag and its `timeoutSec` effect on every live command.
- **`shell.go`** - `hydra-cli shell --config PATH` now persists both the
  resolved server AND timeout as session defaults (`HYDRA_CLI_SERVER`/
  the new `HYDRA_CLI_TIMEOUT_SEC`), the same mechanism `--server` alone
  already used for the server - every command typed afterward inherits
  the config file's target without retyping `--config` each line. A
  `--server`-only session still leaves `HYDRA_CLI_TIMEOUT_SEC` untouched,
  unchanged from before this fix.
- **`tests/`** - real coverage against `net/http/httptest` servers and
  temp-file config fixtures, not mocks: `status`/`robots`/`doctor` each
  gained a test proving `--config` ALONE (no `--server` at all) really
  drives the live HTTP call, plus a real timeout-enforcement test
  (`status`, a handler sleeping 1.2s against a `timeoutSec: 1` config
  file, asserting the request actually aborts around 1s rather than
  waiting out the full handler). `shell` gained a test proving
  `--config` sets both session environment variables. `server_test.go`
  gained precedence tests for the config-file/env-var/flag interplay.
  62/62 tests pass (`go vet`/`go test ./...`/`go build ./...`).
- Still out of scope, on purpose: a standalone `--timeout` flag (only a
  `--config` file sets a custom timeout today) and the fleet-wide
  write commands (`deploy`/`flash-all`/`audit`), both blocked on
  HYDRA-UMC-SERVER's own write endpoints that don't exist yet.

## [0.0.7] - Interactive shell mode

- **`hydra-cli shell [--server URL]`** (`shell.go`, new) - a real REPL:
  reads one command per line and dispatches it through the exact same
  `run()` one-shot invocations use, so shell and one-shot behavior can
  never drift apart. `--server`, if given, sets `HYDRA_CLI_SERVER` for
  the session, reusing the existing environment-variable precedence
  `resolveServer()` already had rather than adding a new one. Real
  quote-aware tokenization (`'...'`/`"..."` group embedded spaces into
  one argument); `exit`/`quit`/Ctrl-D leaves cleanly; nesting a second
  `shell` inside the shell is refused rather than silently recursing.
- 8 new tests covering tokenization, dispatch, EOF handling, the session
  server flag, and the nested-shell refusal - all against a fake
  dispatcher, no real network dependency.
- All 7 READMEs updated.

Verified: `go build`/`go vet`/`go test ./...` all clean, real end-to-end
smoke test of the built binary.

## [0.0.6]

- Build version synchronized with `hydra-umc.project.json` and the repository-native version source.

## [0.0.5] - Stable exit codes, real config validation, honest dry-run apply

- **A real, stable exit-code contract** (`cmd/hydra-cli/exitcode.go`) - every command used to collapse every failure to a bare `exit 1`. Now: `0` ok, `1` unclassified general error, `2` usage error, `3` config error, `4` network error (server unreachable), `5` server error (bad status/response), `6` not implemented. A script wrapping this CLI (CI, a fleet cron job) can branch on *why* a command failed without scraping stderr text, and these codes are meant to stay stable across releases. `status` and `robots` now classify their existing failure paths through this contract instead of an undifferentiated error.
- **`hydra-cli config validate --config PATH`** - real local config file loading (`cmd/hydra-cli/config.go`) and real schema validation: `server` must be a non-empty, absolute URL; `timeoutSec` must be a positive integer. Every real failure mode (missing file, malformed JSON, a value that fails validation) reports a distinct, documented `ExitConfigError`.
- **`hydra-cli config apply --config PATH [--dry-run]`** - the real dry-run-without-a-device operation this gate asks for. `--dry-run` loads and validates the real config, then prints exactly what a live apply would send - proving the real validation path end to end without touching any device or live server. Without `--dry-run` it honestly returns `ExitNotImplemented` rather than pretending to push config to a write endpoint that does not exist yet on HYDRA-UMC-SERVER (see `deploy`/`flash-all` in the roadmap below).
- 24 new tests (`exitcode_test.go`, `config_test.go`) covering every exit-code classification, the full config validation matrix, and both `config apply` paths (dry-run and the honest not-implemented refusal) - 36 tests total, all passing against a real built binary as well as `go test`.

## [0.0.4] - Real `robots` command: a real fleet roster read

- **`hydra-cli robots [--server URL]`** - a real HTTP GET against a live HYDRA-UMC-SERVER's own `GET /api/settings` (already a real, unauthenticated read endpoint - see that project's own `src/server.ts`), printing the real controller/robot roster it reports (name, online status, model, role). The first genuinely "fleet DevOps" command this CLI has, built entirely on an endpoint that already existed rather than needing new server-side work.
- **`src/cmd/hydra-cli/server.go`** - `resolveServer()` factored out of `cmdStatus` so both `status` and `robots` share one real `--server`/`HYDRA_CLI_SERVER`/default resolution instead of two copies that could drift.
- **12 new tests** across `robots_test.go`, `status_test.go`, `server_test.go` - real `net/http/httptest` round-trips (not mocked), including error paths (unreachable server, non-2xx, malformed JSON) and `resolveServer`'s own precedence rules.
- **Real bug found and fixed via an actual end-to-end smoke test**: `robotEntry`/`controllerEntry` originally typed `id` as a Go `int`. Running the real compiled binary against a real, live HYDRA-UMC-SERVER instance (not just the example `data/settings.json` on disk) surfaced a real API inconsistency - a controller's own `id` is a **string** (`"localhost"` for the default local controller) while a robot's own `id` is a **number** - `json.Unmarshal` failed for real on the controller side. Since neither `id` was actually used by the printed output, both were dropped from the structs entirely rather than fought with a custom unmarshaler.
- **`build.sh`/`build.bat`** - now run the real test suite (`go vet` + `go test`) as a required step before compiling; `build.sh`/`build.bat`/`run.sh`/`run.bat` no longer auto-close their window on completion.

## [0.0.0] - Initial scaffolding

- **Go module** (`src/go.mod`, `github.com/JuanenRac/HYDRA-UMC-TOOL-CLI`)
  with a real `cmd/hydra-cli` binary - not a placeholder.
- **`hydra-cli version`** - prints the CLI's own name and version.
- **`hydra-cli help` / `--help` / `-h` / bare invocation** - prints full
  command usage.
- **`hydra-cli status [--server URL]`** - real HTTP client call against a
  HYDRA-UMC-SERVER instance's `GET /api/hydra-info` endpoint (same field
  every other ecosystem client reads - see HYDRA-UMC-STUDIO's About
  dialog); resolves the target from `--server`, then `HYDRA_CLI_SERVER`,
  then `http://localhost:3000`. Fails with a clear error when no server
  is reachable, instead of crashing.
- **`build.sh` / `build.bat`** - bump version, compile, smoke-test the
  binary (`hydra-cli version`).
- **`run.sh` / `run.bat`** - execute the compiled binary, forwarding all
  arguments.
- Fleet-wide operations (`deploy`, `flash-all`, `audit`) described in the
  README are the next milestone; they need
  HYDRA-UMC-SERVER's fleet endpoints to exist first.
