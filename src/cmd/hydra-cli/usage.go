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
    --timeout SECONDS   how long to wait for the server (1 to 3600; default: the config file's "timeoutSec", then HYDRA_CLI_TIMEOUT_SEC, then 5)
`

var commandUsage = map[string]string{
	"version": `USAGE: hydra-cli version [--json]

Print the CLI version and exit.

OPTIONS:
    --json          print the result as one JSON object (schema_version, command, ok and the fields of the command)
`,
	"status": `USAGE: hydra-cli status [--server URL] [--config PATH] [--json]

Query a HYDRA-UMC-SERVER's /api/hydra-info and print its reported identity and version.

OPTIONS:
` + serverOptions + `    --json          print the identity as JSON
`,
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
	"claim": `USAGE: hydra-cli claim <robot> [--ttl-ms N] [--force] [--reason TEXT] [--if-match REVISION|current] [--json]

Reserve a robot for this program. The reservation belongs to your account and to this program's controller id
(--controller-id ID or HYDRA_CLI_CONTROLLER_ID; default hydra-cli-<host>).

OPTIONS:
    --ttl-ms N           how long the reservation lasts
    --force              take it over from another holder (administrators only)
    --controller-id ID   the controller id of this program (default: HYDRA_CLI_CONTROLLER_ID, then hydra-cli-<host>)
    --reason TEXT        written to the audit record
    --if-match REVISION  refuse (exit 8) if the robot changed since REVISION; "current" reads it first
    --json          print the result as one JSON object (schema_version, command, ok and the fields of the command)
` + serverOptions,
	"release": `USAGE: hydra-cli release <robot> [--reason TEXT] [--if-match REVISION|current] [--json]

Release a robot this program reserved.

OPTIONS:
    --reason TEXT        written to the audit record
    --controller-id ID   the controller id of this program (default: HYDRA_CLI_CONTROLLER_ID, then hydra-cli-<host>)
    --if-match REVISION  refuse (exit 8) if the robot changed since REVISION; "current" reads it first
    --json          print the result as one JSON object (schema_version, command, ok and the fields of the command)
` + serverOptions,
	"limits": `USAGE: hydra-cli limits [--robot ID] [--json]

Read the safety limits the server holds: jog size, absolute jog range, bed temperature, jog arming.

OPTIONS:
    --robot ID      the limits of one robot's model
    --json          print them as JSON
` + serverOptions,
	"jog-arm": `USAGE: hydra-cli jog-arm <robot> <on|off> [--ttl-ms N] [--json]

Arm or disarm the jog of one robot. Needed only when the server requires it; it ends by itself, with a stop or when
the robot is released.

OPTIONS:
    --ttl-ms N      how long the arming lasts
    --json          print the result as one JSON object (schema_version, command, ok and the fields of the command)
` + serverOptions,
	"jog": `USAGE: hydra-cli jog <robot> <axis> <amount> [--absolute] [--if-match REVISION|current] [--json]

Move one axis (x y z a b c). A step above the limit of the robot's model is refused before it is sent.

OPTIONS:
    --absolute           the amount is a position, not a step
    --if-match REVISION  refuse (exit 8) if the robot changed since REVISION; "current" reads it first
    --json          print the result as one JSON object (schema_version, command, ok and the fields of the command)
` + serverOptions,
	"table": `USAGE: hydra-cli table <robot> <x> <y> [--rz N] [--relative] [--if-match REVISION|current] [--json]

Move the XY table of a robot in one atomic request (absolute positions unless --relative). The whole move is checked
against the limits of the model before it is sent; the server applies all of it or none of it.

OPTIONS:
    --rz N               rotate to N as part of the same move
    --relative           x and y are steps, not positions
    --if-match REVISION  refuse (exit 8) if the robot changed since REVISION; "current" reads it first
    --json          print the result as one JSON object (schema_version, command, ok and the fields of the command)
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
	"config": `USAGE: hydra-cli config validate --config PATH [--json]
       hydra-cli config apply --config PATH [--dry-run] [--json]

validate loads a local config file and checks it against the schema. apply previews (--dry-run) what it would send;
without --dry-run it exits 6 (not implemented): no live fleet-write endpoint exists yet.
--json prints the result as one JSON object.
`,
	"shell": `USAGE: hydra-cli shell [--server URL] [--config PATH]

Interactive REPL: run any command repeatedly against the same server without restarting the process. exit, quit or
Ctrl-D to leave.

OPTIONS:
` + serverOptions,
	"completion": `USAGE: hydra-cli completion <bash|zsh|fish|powershell>

Print a completion script for the shell to stdout: eval "$(hydra-cli completion bash)" enables tab-completion.
For fish: hydra-cli completion fish | source. For PowerShell: hydra-cli completion powershell | Out-String | Invoke-Expression.
`,
	"login": `USAGE: hydra-cli login [--user NAME] [--password-stdin] [--json]

Sign in once and save the session (the token and its refresh token) in a file only you can read. Every later command
uses it and renews it by itself, so no command logs in again. The file is HYDRA_CLI_TOKEN_FILE, the "tokenFile" of a
--config file, or hydra-cli/session.json in your configuration directory. The password is read from the environment
(HYDRA_CLI_PASSWORD) or, with --password-stdin, from the first line of the standard input: never from an option.

OPTIONS:
    --user NAME     the account (default: HYDRA_CLI_USER)
    --password-stdin  read the password from the standard input
    --json          print the result as one JSON object (never the token)
` + serverOptions,
	"logout": `USAGE: hydra-cli logout [--json]

End the saved session: the server revokes the refresh token and the session file is removed.

OPTIONS:
    --json          print the result as one JSON object
` + serverOptions,
	"scan": `USAGE: hydra-cli scan <network|address>... [--ports 3000,3443] [--scheme http|https] [--json]

Find HYDRA-UMC-SERVERs: every address is asked for /api/hydra-info (public; no credential is sent). Only private networks
are scanned unless --allow-public-range is given, and no more than --max-hosts addresses.

OPTIONS:
    --ports LIST        ports to try, separated by commas (default 3000)
    --scheme SCHEME     http or https (default http)
    --concurrency N     probes at once, 1 to 256 (default 64)
    --max-hosts N       most addresses to probe, up to 65536 (default 1024)
    --probe-timeout-ms N  how long to wait for one address, 50 to 10000 (default 800)
    --allow-public-range  allow addresses that are not on a private network
    --json              print the servers found as JSON
`,
	"fleet": `USAGE: hydra-cli fleet doctor --servers FILE [--concurrency N] [--json]

Run the read-only doctor against every server of a list at once and give one answer. The file has one server URL per
line; blank lines and lines starting with # are skipped. Exit 0 when every server passes; otherwise the exit code the
failures share, or 1 when they differ.

OPTIONS:
    --servers FILE    the list of servers
    --concurrency N   servers at once, 1 to 64 (default 8)
    --json            print one report per server
    --config PATH     a config file for the timeout and the transport options
    --timeout SECONDS how long to wait for each server
`,
	"stop": `USAGE: hydra-cli stop <robot> [--controller-id ID] [--json]

Send the stop command to one robot. The server lets it through even when another account reserved the robot.
This is a software command, not the emergency stop of the cell.

OPTIONS:
    --controller-id ID   the controller id of this program (default: HYDRA_CLI_CONTROLLER_ID, then hydra-cli-<host>)
    --json          print the result as one JSON object (schema_version, command, ok and the fields of the command)
` + serverOptions,
	"stop-all": `USAGE: hydra-cli stop-all --yes [--json]

The global stop: every robot of the server, whoever reserved it, and every valve and pump switched off. It reaches robots
of other accounts, so it asks for --yes. This is a software command, not the emergency stop of the cell.

OPTIONS:
    --yes           confirm (without it nothing is sent)
    --json          print the answer of the server as JSON
` + serverOptions,
	"metrics": `USAGE: hydra-cli metrics [--all] [--json]

Read GET /metrics of the server and show uptime, clients, commands sent, login failures, CPU load, memory and temperature.

OPTIONS:
    --all           show every sample
    --json          print the samples as JSON
` + serverOptions,
	"manifest": `USAGE: hydra-cli manifest validate [PATH...] [--workspace DIR] [--schema FILE] [--json]

Check hydra-umc.project.json files against the schema of the SDK (embedded in this build). PATH is a manifest or a
repository directory; --workspace DIR checks every HYDRA-UMC-* and URTC-* directory in DIR; with neither, the
current directory. A manifest whose name is not the name of its directory fails too. Exit 3 when any fails.

OPTIONS:
    --workspace DIR  check every repository in DIR
    --schema FILE    use another schema file
    --json           print the verdicts as JSON
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
