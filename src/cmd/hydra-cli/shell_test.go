// =============================================================================
// HYDRA-UMC-TOOL-CLI - Interactive REPL tests: cmd/hydra-cli/shell_test.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0 - see LICENSE
// =============================================================================
package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestSplitShellWords(t *testing.T) {
	cases := []struct {
		name  string
		line  string
		want  []string
		error bool
	}{
		{name: "simple", line: "status --server http://x:3000", want: []string{"status", "--server", "http://x:3000"}},
		{name: "double_quoted_with_space", line: `config validate --config "my file.json"`, want: []string{"config", "validate", "--config", "my file.json"}},
		{name: "single_quoted_with_space", line: `config validate --config 'my file.json'`, want: []string{"config", "validate", "--config", "my file.json"}},
		{name: "collapses_repeated_whitespace", line: "  status   robots  ", want: []string{"status", "robots"}},
		{name: "empty_line", line: "", want: nil},
		{name: "unterminated_quote", line: `config validate --config "no closing quote`, error: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := splitShellWords(tc.line)
			if tc.error {
				if err == nil {
					t.Fatalf("expected an error, got tokens %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestCmdShell_DispatchesEachLineAndStopsOnExit(t *testing.T) {
	var received [][]string
	fakeDispatch := func(args []string) ExitCode {
		received = append(received, args)
		return ExitOK
	}

	in := strings.NewReader("version\nstatus --server http://example.invalid:1\nexit\n")
	var out bytes.Buffer

	err := cmdShell(&out, in, nil, fakeDispatch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(received) != 2 {
		t.Fatalf("expected 2 dispatched lines, got %d: %v", len(received), received)
	}
	if received[0][0] != "version" {
		t.Fatalf("first command = %v, want [version]", received[0])
	}
	if received[1][0] != "status" {
		t.Fatalf("second command = %v, want to start with status", received[1])
	}
}

func TestCmdShell_StopsCleanlyOnEOF(t *testing.T) {
	fakeDispatch := func(args []string) ExitCode { return ExitOK }
	in := strings.NewReader("version\n") // no exit/quit line - just runs out
	var out bytes.Buffer

	if err := cmdShell(&out, in, nil, fakeDispatch); err != nil {
		t.Fatalf("unexpected error on EOF: %v", err)
	}
}

func TestCmdShell_SkipsBlankLinesAndUnknownServerFlagUsage(t *testing.T) {
	fakeDispatch := func(args []string) ExitCode { return ExitOK }
	in := strings.NewReader("\n   \nquit\n")
	var out bytes.Buffer

	if err := cmdShell(&out, in, nil, fakeDispatch); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdShell_RejectsExtraArguments(t *testing.T) {
	fakeDispatch := func(args []string) ExitCode { return ExitOK }
	var out bytes.Buffer

	err := cmdShell(&out, strings.NewReader(""), []string{"unexpected"}, fakeDispatch)
	if err == nil {
		t.Fatal("expected an error for an unrecognized extra argument")
	}
}

func TestCmdShell_SetsSessionServerFromFlag(t *testing.T) {
	original, hadOriginal := os.LookupEnv("HYDRA_CLI_SERVER")
	defer func() {
		if hadOriginal {
			os.Setenv("HYDRA_CLI_SERVER", original)
		} else {
			os.Unsetenv("HYDRA_CLI_SERVER")
		}
	}()
	os.Unsetenv("HYDRA_CLI_SERVER")

	fakeDispatch := func(args []string) ExitCode { return ExitOK }
	var out bytes.Buffer

	err := cmdShell(&out, strings.NewReader("quit\n"), []string{"--server", "http://session-target:9000"}, fakeDispatch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := os.Getenv("HYDRA_CLI_SERVER"); got != "http://session-target:9000" {
		t.Fatalf("HYDRA_CLI_SERVER = %q, want the session --server value", got)
	}
}

func TestCmdShell_RefusesToNestAnotherShell(t *testing.T) {
	var received [][]string
	fakeDispatch := func(args []string) ExitCode {
		received = append(received, args)
		return ExitOK
	}
	var out bytes.Buffer

	err := cmdShell(&out, strings.NewReader("shell\nquit\n"), nil, fakeDispatch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(received) != 0 {
		t.Fatalf("expected 'shell' to be intercepted before dispatch, got %v", received)
	}
	if !strings.Contains(out.String(), "already in an interactive shell") {
		t.Fatalf("expected a refusal notice, got output: %q", out.String())
	}
}
