# Changelog

All notable work on **HYDRA-UMC-TOOL-CLI** is summarized here, newest first.
Full session-by-session detail (including dates) lives in a private,
unpublished internal log - this file is public, so it intentionally omits
calendar dates.

## Versioning scheme

`src/cmd/hydra-cli/version.go`'s `Version` constant bumps automatically on
every real build (`build.sh` / `build.bat` - see `bump_version.py`, run as
the first step of both scripts). It follows the same base-10 "odometer"
rule used across the ecosystem rather than semantic-versioning judgment
calls:

- `patch` +1 on every build
- when `patch` would exceed 9, it resets to 0 and `minor` +1 instead (e.g. `0.0.9` -> `0.1.0`, never `0.0.10`)
- the same carry cascades into `major` if `minor` would exceed 9

`hydra-cli version` always prints the exact value currently baked into the
binary.

---

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
