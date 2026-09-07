// =============================================================================
// HYDRA-UMC-TOOL-CLI - Interactive REPL: cmd/hydra-cli/shell.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0 - see LICENSE
// =============================================================================
// `hydra-cli shell` reads one command per line from stdin and dispatches it
// through the exact same run() every one-shot invocation of this binary
// uses - no second copy of the command table, so shell and one-shot
// behavior can never drift apart. Its only real job beyond that dispatch
// is letting an operator set the target server/timeout once per session
// instead of repeating --server/--config on every line, by reusing the
// existing HYDRA_CLI_SERVER/HYDRA_CLI_TIMEOUT_SEC environment variables
// resolveTarget() (server.go) already reads - not a new precedence rule.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

// runFunc matches run()'s own signature - a plain function value so
// cmdShell can be tested against a fake dispatcher instead of a real
// network-touching command table.
type runFunc func(args []string) ExitCode

// cmdShell implements `hydra-cli shell [--server URL] [--config PATH]`.
// Reads lines from in until EOF (Ctrl-D) or an "exit"/"quit" line,
// writing its prompt and any shell-level notices to out. dispatch is the
// real per-line command runner - production always passes run itself.
func cmdShell(out io.Writer, in io.Reader, args []string, dispatch runFunc) error {
	server, timeout, rest, err := resolveTarget(args)
	if err != nil {
		return err
	}
	if len(rest) != 0 {
		return newCliError(ExitUsageError, fmt.Errorf("shell does not accept arguments: %s", strings.Join(rest, " ")))
	}
	// Only override a session default when the caller actually passed the
	// matching flag - resolveTarget() already fell back to
	// HYDRA_CLI_SERVER/HYDRA_CLI_TIMEOUT_SEC or its own compiled-in
	// defaults on its own, and re-setting an environment variable to that
	// same resolved value would needlessly shadow a value the user's real
	// shell environment set before launching this binary. Only --config
	// ever carries a real timeout value (there is no standalone --timeout
	// flag), so HYDRA_CLI_TIMEOUT_SEC is only ever persisted alongside it -
	// a --server-only session leaves it untouched, same as before this
	// --config wiring existed.
	if hasServerFlag(args) || hasConfigFlag(args) {
		if err := os.Setenv("HYDRA_CLI_SERVER", server); err != nil {
			return newCliError(ExitGeneralError, fmt.Errorf("could not set session server: %w", err))
		}
	}
	if hasConfigFlag(args) {
		if err := os.Setenv("HYDRA_CLI_TIMEOUT_SEC", strconv.Itoa(int(timeout/time.Second))); err != nil {
			return newCliError(ExitGeneralError, fmt.Errorf("could not set session timeout: %w", err))
		}
	}

	fmt.Fprintf(out, "%s v%s interactive shell - target %s\n", projectName, Version, server)
	fmt.Fprintln(out, "Type a command (version, status, robots, doctor, config ...), or exit/quit to leave.")

	scanner := bufio.NewScanner(in)
	for {
		fmt.Fprint(out, "hydra-cli> ")
		if !scanner.Scan() {
			fmt.Fprintln(out)
			return nil // real EOF (Ctrl-D / piped input exhausted) - a clean, non-error exit.
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "exit" || line == "quit" {
			return nil
		}

		tokens, err := splitShellWords(line)
		if err != nil {
			fmt.Fprintf(out, "hydra-cli: %v\n", err)
			continue
		}
		if len(tokens) == 0 {
			continue
		}
		if tokens[0] == "shell" {
			// A real, if unusual, request (nesting a shell inside a shell) -
			// refused explicitly rather than silently recursing into a
			// second read loop over the same stdin, which would just look
			// like this shell hung.
			fmt.Fprintln(out, "hydra-cli: already in an interactive shell")
			continue
		}
		dispatch(tokens)
	}
}

// splitShellWords tokenizes one line the way an operator expects a shell
// to: whitespace-separated, with '...' and "..." each grouping their own
// contents (including embedded whitespace) into a single token - real for
// a --config path or a value containing a space, not just the common case.
// Returns an error for an unterminated quote instead of silently dropping
// or misparsing the rest of the line.
func splitShellWords(line string) ([]string, error) {
	var (
		tokens  []string
		current strings.Builder
		inWord  bool
		quote   rune
	)
	flush := func() {
		if inWord {
			tokens = append(tokens, current.String())
			current.Reset()
			inWord = false
		}
	}
	for _, r := range line {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
			inWord = true
		case r == ' ' || r == '\t':
			flush()
		default:
			inWord = true
			current.WriteRune(r)
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated %c quote", quote)
	}
	flush()
	return tokens, nil
}

// hasServerFlag reports whether args explicitly passed --server, as
// opposed to resolveTarget() having only fallen back to the environment
// variable or the compiled-in default.
func hasServerFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--server" {
			return true
		}
	}
	return false
}

// hasConfigFlag reports whether args explicitly passed --config, as
// opposed to resolveTarget() having only fallen back to the environment
// variables or the compiled-in defaults.
func hasConfigFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--config" {
			return true
		}
	}
	return false
}
