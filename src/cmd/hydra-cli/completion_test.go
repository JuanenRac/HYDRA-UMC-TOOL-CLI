// =============================================================================
// HYDRA-UMC-TOOL-CLI - cmd/hydra-cli/completion_test.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0 - see LICENSE
// =============================================================================
package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestCmdCompletion_BashListsEveryRealTopLevelCommand(t *testing.T) {
	var buf bytes.Buffer
	if err := cmdCompletion(&buf, []string{"bash"}); err != nil {
		t.Fatalf("cmdCompletion(bash) returned an error: %v", err)
	}
	out := buf.String()
	for _, cmd := range topLevelCommands {
		if !strings.Contains(out, cmd) {
			t.Errorf("bash completion script is missing real command %q", cmd)
		}
	}
	if !strings.Contains(out, "complete -F _hydra_cli_complete hydra-cli") {
		t.Error("bash completion script never registers itself with `complete`")
	}
}

func TestCmdCompletion_ZshListsEveryRealTopLevelCommand(t *testing.T) {
	var buf bytes.Buffer
	if err := cmdCompletion(&buf, []string{"zsh"}); err != nil {
		t.Fatalf("cmdCompletion(zsh) returned an error: %v", err)
	}
	out := buf.String()
	for _, cmd := range topLevelCommands {
		if !strings.Contains(out, cmd) {
			t.Errorf("zsh completion script is missing real command %q", cmd)
		}
	}
	if !strings.Contains(out, "compdef _hydra_cli hydra-cli") {
		t.Error("zsh completion script never registers itself with compdef")
	}
}

func TestCmdCompletion_RejectsAnUnsupportedShellAsUsageError(t *testing.T) {
	var buf bytes.Buffer
	err := cmdCompletion(&buf, []string{"fish"})
	if err == nil {
		t.Fatal("expected an error for an unsupported shell, got nil")
	}
	if got := exitCodeFor(err); got != ExitUsageError {
		t.Fatalf("exitCodeFor(unsupported shell) = %d, want ExitUsageError", got)
	}
}

func TestCmdCompletion_RejectsMissingArgumentAsUsageError(t *testing.T) {
	var buf bytes.Buffer
	err := cmdCompletion(&buf, []string{})
	if err == nil {
		t.Fatal("expected an error when no shell is given, got nil")
	}
	if got := exitCodeFor(err); got != ExitUsageError {
		t.Fatalf("exitCodeFor(missing shell) = %d, want ExitUsageError", got)
	}
}

func TestCmdCompletion_ConfigSubcommandsAreListedForBothShells(t *testing.T) {
	for _, shell := range []string{"bash", "zsh"} {
		var buf bytes.Buffer
		if err := cmdCompletion(&buf, []string{shell}); err != nil {
			t.Fatalf("cmdCompletion(%s) returned an error: %v", shell, err)
		}
		out := buf.String()
		for _, sub := range configSubcommands {
			if !strings.Contains(out, sub) {
				t.Errorf("%s completion script is missing real config subcommand %q", shell, sub)
			}
		}
	}
}
