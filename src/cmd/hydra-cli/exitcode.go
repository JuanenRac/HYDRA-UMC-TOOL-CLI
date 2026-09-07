// =============================================================================
// HYDRA-UMC-TOOL-CLI - Real, stable exit code contract: cmd/hydra-cli/exitcode.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0 - see LICENSE
// =============================================================================
// Before this, every command's failure collapsed to the same bare exit 1 -
// a script wrapping this CLI (a CI pipeline, a fleet cron job) could tell
// "it worked" from "it didn't", but never WHY, without scraping stderr
// text. These codes are the real, documented contract every command in
// this binary now returns through, and are meant to stay stable across
// releases - a caller can branch on them without parsing a human-readable
// message that's free to change wording.
package main

import "errors"

type ExitCode int

const (
	ExitOK ExitCode = 0
	// ExitGeneralError is the fallback for a real failure this binary
	// did not classify more specifically - kept distinct from the codes
	// below so "we don't yet have a precise reason" is itself honest,
	// not silently folded into one of the specific categories.
	ExitGeneralError ExitCode = 1
	// ExitUsageError: the invocation itself was wrong (unknown command,
	// missing/malformed arguments) - the same real command run correctly
	// would not fail this way. 2 matches the common Unix CLI convention
	// for usage errors (e.g. `grep`, `git`).
	ExitUsageError ExitCode = 2
	// ExitConfigError: a local config file (see config.go) failed real
	// schema validation - nothing on the network was even attempted.
	ExitConfigError ExitCode = 3
	// ExitNetworkError: the target HYDRA-UMC-SERVER could not be reached
	// at all (DNS/connection/timeout) - distinct from the server being
	// reachable but replying with a real error.
	ExitNetworkError ExitCode = 4
	// ExitServerError: HYDRA-UMC-SERVER was reached but replied with a
	// non-2xx status or a response this CLI could not actually parse.
	ExitServerError ExitCode = 5
	// ExitNotImplemented: a real, honest "this operation genuinely
	// cannot run live yet" outcome (e.g. `config apply` without
	// --dry-run - see config.go) - distinct from every error above,
	// which all mean something actually went wrong.
	ExitNotImplemented ExitCode = 6
)

// CliError carries a real ExitCode alongside a normal Go error, so a
// command function can keep returning a plain `error` (idiomatic,
// wrappable with fmt.Errorf/%w) while still telling main() which stable
// exit code the failure maps to.
type CliError struct {
	Code ExitCode
	Err  error
}

func (e *CliError) Error() string { return e.Err.Error() }
func (e *CliError) Unwrap() error { return e.Err }

func newCliError(code ExitCode, err error) *CliError {
	return &CliError{Code: code, Err: err}
}

// exitCodeFor maps any error a command returns to its real, stable exit
// code: a *CliError reports its own real classification; anything else
// (an error a command returned but never explicitly classified) falls
// back to ExitGeneralError rather than guessing.
func exitCodeFor(err error) ExitCode {
	if err == nil {
		return ExitOK
	}
	var cliErr *CliError
	if errors.As(err, &cliErr) {
		return cliErr.Code
	}
	return ExitGeneralError
}
