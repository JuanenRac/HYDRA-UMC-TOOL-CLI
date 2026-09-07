// =============================================================================
// HYDRA-UMC-TOOL-CLI - cmd/hydra-cli/exitcode_test.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0 - see LICENSE
// =============================================================================
package main

import (
	"errors"
	"fmt"
	"testing"
)

func TestExitCodeFor_Nil(t *testing.T) {
	if got := exitCodeFor(nil); got != ExitOK {
		t.Fatalf("exitCodeFor(nil) = %d, want ExitOK", got)
	}
}

func TestExitCodeFor_UnclassifiedErrorFallsBackToGeneral(t *testing.T) {
	if got := exitCodeFor(errors.New("boom")); got != ExitGeneralError {
		t.Fatalf("exitCodeFor(plain error) = %d, want ExitGeneralError", got)
	}
}

func TestExitCodeFor_CliErrorReportsItsOwnCode(t *testing.T) {
	err := newCliError(ExitConfigError, errors.New("bad config"))
	if got := exitCodeFor(err); got != ExitConfigError {
		t.Fatalf("exitCodeFor(CliError) = %d, want ExitConfigError", got)
	}
}

func TestExitCodeFor_WrappedCliErrorStillClassifies(t *testing.T) {
	base := newCliError(ExitNetworkError, errors.New("unreachable"))
	wrapped := fmt.Errorf("status: %w", base)
	if got := exitCodeFor(wrapped); got != ExitNetworkError {
		t.Fatalf("exitCodeFor(wrapped CliError) = %d, want ExitNetworkError", got)
	}
}

func TestCliError_ErrorAndUnwrap(t *testing.T) {
	inner := errors.New("root cause")
	err := newCliError(ExitServerError, inner)

	if err.Error() != inner.Error() {
		t.Fatalf("Error() = %q, want %q", err.Error(), inner.Error())
	}
	if !errors.Is(err, inner) {
		t.Fatal("errors.Is(err, inner) = false, want true (Unwrap must expose the root cause)")
	}
}

func TestRun_UnknownCommandIsUsageError(t *testing.T) {
	if got := run([]string{"definitely-not-a-real-command"}); got != ExitUsageError {
		t.Fatalf("run(unknown command) = %d, want ExitUsageError", got)
	}
}

func TestRun_NoArgsPrintsHelpAndExitsOK(t *testing.T) {
	if got := run(nil); got != ExitOK {
		t.Fatalf("run(no args) = %d, want ExitOK", got)
	}
}

func TestRun_VersionExitsOK(t *testing.T) {
	if got := run([]string{"version"}); got != ExitOK {
		t.Fatalf("run(version) = %d, want ExitOK", got)
	}
}

func TestRun_StatusUnreachableServerIsNetworkError(t *testing.T) {
	got := run([]string{"status", "--server", "http://127.0.0.1:1"})
	if got != ExitNetworkError {
		t.Fatalf("run(status, unreachable) = %d, want ExitNetworkError", got)
	}
}
