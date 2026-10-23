// =============================================================================
// HYDRA-UMC-TOOL-CLI - Help of one command: cmd/hydra-cli/usage.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0 - see LICENSE
// =============================================================================
// `hydra-cli <command> --help` (or -h) prints the usage of that command alone and exits 0, whatever else was
// typed after it and without touching the network or any file. The general help lists every command; this is
// the same text for one of them, so a script or a person does not have to read the whole page.
package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// serverOptions are the three ways to point a command at a server, shared by every live command.
const serverOptions = `    --server URL    the server (default: HYDRA_CLI_SERVER, then a --config file's "server", then ` + defaultServerURL + `)
    --config PATH   a config file (see: hydra-cli config validate --help)
`

var commandUsage = map[string]string{
	"version": `USAGE: hydra-cli version

Print the CLI version and exit.
`,
	"status": `USAGE: hydra-cli status [--server URL] [--config PATH]

Query a HYDRA-UMC-SERVER's /api/hydra-info and print its reported identity and version.

OPTIONS:
` + serverOptions,
	"robots": `USAGE: hydra-cli robots [--server URL] [--config PATH] [--json]

Print the controller and robot roster the server reports (/api/settings). Needs HYDRA_CLI_TOKEN, or
HYDRA_CLI_USER and HYDRA_CLI_PASSWORD, in the environment.

OPTIONS:
` + serverOptions + `    --json          print the roster as JSON
`,
	"doctor": `USAGE: hydra-cli doctor [--server URL] [--config PATH] [--json]

Read-only diagnostic: validates /api/hydra-info and /api/settings and checks that their controller and robot counts
agree. It never sends a command or probes hardware.

OPTIONS:
` + serverOptions + `    --json          print a structured report (checkId, severity, status per check) instead of DOCTOR=PASS
`,
	"claims": `USAGE: hydra-cli claims [--server URL] [--config PATH] [--json]

List who reserves which robot (GET /api/claims).

OPTIONS:
` + serverOptions + `    --json          print the list as JSON
`,
	"claim": `USAGE: hydra-cli claim <robot> [--ttl-ms N] [--force] [--reason TEXT] [--if-match REVISION|current]

Reserve a robot for this program. The reservation belongs to your account and to this program's controller id
(--controller-id ID or HYDRA_CLI_CONTROLLER_ID; default hydra-cli-<host>).

OPTIONS:
    --ttl-ms N           how long the reservation lasts
    --force              take it over from another holder (administrators only)
    --controller-id ID   the controller id of this program (default: HYDRA_CLI_CONTROLLER_ID, then hydra-cli-<host>)
    --reason TEXT        written to the audit record
    --if-match REVISION  refuse (exit 8) if the robot changed since REVISION; "current" reads it first
` + serverOptions,
	"release": `USAGE: hydra-cli release <robot> [--reason TEXT] [--if-match REVISION|current]

Release a robot this program reserved.

OPTIONS:
    --reason TEXT        written to the audit record
    --controller-id ID   the controller id of this program (default: HYDRA_CLI_CONTROLLER_ID, then hydra-cli-<host>)
    --if-match REVISION  refuse (exit 8) if the robot changed since REVISION; "current" reads it first
` + serverOptions,
	"limits": `USAGE: hydra-cli limits [--robot ID] [--json]

Read the safety limits the server holds: jog size, absolute jog range, bed temperature, jog arming.

OPTIONS:
    --robot ID      the limits of one robot's model
    --json          print them as JSON
` + serverOptions,
	"jog-arm": `USAGE: hydra-cli jog-arm <robot> <on|off> [--ttl-ms N]

Arm or disarm the jog of one robot. Needed only when the server requires it; it ends by itself, with a stop or when
the robot is released.

OPTIONS:
    --ttl-ms N      how long the arming lasts
` + serverOptions,
	"jog": `USAGE: hydra-cli jog <robot> <axis> <amount> [--absolute] [--if-match REVISION|current]

Move one axis (x y z a b c). A step above the limit of the robot's model is refused before it is sent.

OPTIONS:
    --absolute           the amount is a position, not a step
    --if-match REVISION  refuse (exit 8) if the robot changed since REVISION; "current" reads it first
` + serverOptions,
	"table": `USAGE: hydra-cli table <robot> <x> <y> [--rz N] [--relative] [--if-match REVISION|current]

Move the XY table of a robot in one atomic request (absolute positions unless --relative). The whole move is checked
against the limits of the model before it is sent; the server applies all of it or none of it.

OPTIONS:
    --rz N               rotate to N as part of the same move
    --relative           x and y are steps, not positions
    --if-match REVISION  refuse (exit 8) if the robot changed since REVISION; "current" reads it first
` + serverOptions,
	"robot": `USAGE: hydra-cli robot <robot> [--json]

Read one robot (GET /api/robot/:id) with its revision, the number --if-match takes.

OPTIONS:
    --json          print it as JSON
` + serverOptions,
	"audit": `USAGE: hydra-cli audit [--limit N] [--type T] [--robot ID] [--verify] [--export FILE] [--json]

Read the audit record (administrators only): the newest lines, with the chain of SHA-256 hashes checked here and,
with --verify, by the server over the whole record.

OPTIONS:
    --limit N       how many lines
    --type T        only lines of this type
    --robot ID      only lines of this robot
    --verify        ask the server to verify the whole chain
    --export FILE   save the lines as stored (.jsonl)
    --json          print the answer as JSON
` + serverOptions,
	"config": `USAGE: hydra-cli config validate --config PATH
       hydra-cli config apply --config PATH [--dry-run]

validate loads a local config file and checks it against the schema. apply previews (--dry-run) what it would send;
without --dry-run it exits 6 (not implemented): no live fleet-write endpoint exists yet.
`,
	"shell": `USAGE: hydra-cli shell [--server URL] [--config PATH]

Interactive REPL: run any command repeatedly against the same server without restarting the process. exit, quit or
Ctrl-D to leave.

OPTIONS:
` + serverOptions,
	"completion": `USAGE: hydra-cli completion <bash|zsh>

Print a completion script for the shell to stdout: eval "$(hydra-cli completion bash)" enables tab-completion.
`,
}

// commandNames are the commands that have a usage text, sorted.
func commandNames() []string {
	names := make([]string, 0, len(commandUsage))
	for name := range commandUsage {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// wantsCommandHelp is true when the arguments after a command ask for its help.
func wantsCommandHelp(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return true
		}
	}
	return false
}

// printCommandUsage writes the usage of one command; false when there is none.
func printCommandUsage(w io.Writer, name string) bool {
	text, ok := commandUsage[name]
	if !ok {
		return false
	}
	fmt.Fprint(w, strings.TrimLeft(text, "\n"))
	return true
}
