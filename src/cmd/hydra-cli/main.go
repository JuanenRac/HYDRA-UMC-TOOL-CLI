// =============================================================================
// HYDRA-UMC-TOOL-CLI - Command-line entry point: cmd/hydra-cli/main.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0 - see LICENSE
// =============================================================================
// The DevOps Swiss-army-knife for the HYDRA-UMC/URTC ecosystem. A real,
// functional command dispatcher: version/help/status,
// plus `robots` - a real read of HYDRA-UMC-SERVER's own live fleet
// roster (see robots.go). The heavier WRITE fleet operations described
// in the README (deploy/flash-all/audit against a live swarm) still land
// in later passes, since they need write endpoints on HYDRA-UMC-SERVER
// that do not exist yet.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

const projectName = "HYDRA-UMC-TOOL-CLI"

// defaultServerURL matches HYDRA-UMC-SERVER's own default bind address
// (see HYDRA-UMC-SERVER/src/server.ts) - overridable so this CLI works
// against any deployment without a rebuild.
const defaultServerURL = "http://localhost:3000"

func main() {
	os.Exit(int(run(os.Args[1:])))
}

// run is main()'s real body, split out so it returns an ExitCode instead
// of calling os.Exit directly - every command's failure now flows
// through the same real, stable classification (see exitcode.go) instead
// of the ad-hoc "always exit 1" this dispatcher used before.
func run(args []string) ExitCode {
	if len(args) == 0 {
		printHelp()
		return ExitOK
	}

	switch args[0] {
	case "version", "-v", "--version":
		cmdVersion()
		return ExitOK
	case "help", "-h", "--help":
		printHelp()
		return ExitOK
	case "status":
		if err := cmdStatus(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "hydra-cli status: %v\n", err)
			return exitCodeFor(err)
		}
		return ExitOK
	case "robots":
		if err := cmdRobots(os.Stdout, args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "hydra-cli robots: %v\n", err)
			return exitCodeFor(err)
		}
		return ExitOK
	case "doctor":
		if err := cmdDoctor(os.Stdout, args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "hydra-cli doctor: %v\n", err)
			return exitCodeFor(err)
		}
		return ExitOK
	case "claims":
		if err := cmdClaims(os.Stdout, args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "hydra-cli claims: %v\n", err)
			return exitCodeFor(err)
		}
		return ExitOK
	case "claim":
		if err := cmdClaim(os.Stdout, args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "hydra-cli claim: %v\n", err)
			return exitCodeFor(err)
		}
		return ExitOK
	case "release":
		if err := cmdRelease(os.Stdout, args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "hydra-cli release: %v\n", err)
			return exitCodeFor(err)
		}
		return ExitOK
	case "limits":
		if err := cmdLimits(os.Stdout, args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "hydra-cli limits: %v\n", err)
			return exitCodeFor(err)
		}
		return ExitOK
	case "jog-arm":
		if err := cmdJogArm(os.Stdout, args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "hydra-cli jog-arm: %v\n", err)
			return exitCodeFor(err)
		}
		return ExitOK
	case "jog":
		if err := cmdJog(os.Stdout, args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "hydra-cli jog: %v\n", err)
			return exitCodeFor(err)
		}
		return ExitOK
	case "table":
		if err := cmdTable(os.Stdout, args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "hydra-cli table: %v\n", err)
			return exitCodeFor(err)
		}
		return ExitOK
	case "robot":
		if err := cmdRobot(os.Stdout, args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "hydra-cli robot: %v\n", err)
			return exitCodeFor(err)
		}
		return ExitOK
	case "audit":
		if err := cmdAudit(os.Stdout, args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "hydra-cli audit: %v\n", err)
			return exitCodeFor(err)
		}
		return ExitOK
	case "config":
		if err := cmdConfig(os.Stdout, args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "hydra-cli config: %v\n", err)
			return exitCodeFor(err)
		}
		return ExitOK
	case "shell":
		if err := cmdShell(os.Stdout, os.Stdin, args[1:], run); err != nil {
			fmt.Fprintf(os.Stderr, "hydra-cli shell: %v\n", err)
			return exitCodeFor(err)
		}
		return ExitOK
	case "completion":
		if err := cmdCompletion(os.Stdout, args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "hydra-cli completion: %v\n", err)
			return exitCodeFor(err)
		}
		return ExitOK
	default:
		fmt.Fprintf(os.Stderr, "hydra-cli: unknown command %q\n\n", args[0])
		printHelp()
		return ExitUsageError
	}
}

// cmdVersion implements `hydra-cli version` - prints name and version and
// exits 0. Real, functional, no network or file dependency.
func cmdVersion() {
	fmt.Printf("%s v%s\n", projectName, Version)
}

// printHelp implements `hydra-cli help` / `hydra-cli --help` / bare
// `hydra-cli` - lists every real subcommand this binary supports today.
func printHelp() {
	fmt.Printf(`%s v%s
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
                          Both default to %s; override with --server, a
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
    table <robot> <x> <y> [--rz N] [--relative]
                          Move the XY table of a robot in one atomic request
                          (absolute positions unless --relative). The whole move
                          is checked against the limits of the model before it
                          is sent; the server applies all of it or none of it.
    robot <robot> [--json]
                          Read one robot (GET /api/robot/:id) with its revision.
    audit [--limit N] [--type T] [--robot ID] [--verify] [--export FILE] [--json]
                          Read the audit record (administrators only): the
                          newest lines, with the chain of SHA-256 hashes checked
                          here and, with --verify, by the server over the whole
                          record. --export saves the lines as stored (.jsonl).

    claim, release, jog and table also take --if-match REVISION (a revision
    printed by robot or by an earlier command) or --if-match current: the
    server then refuses the write (exit code 8) if the robot changed since.
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
`, projectName, Version, defaultServerURL)
}

// hydraInfo mirrors the subset of HYDRA-UMC-SERVER's GET /api/hydra-info
// response this CLI actually reads. Unknown fields are ignored, so this
// stays forward-compatible with a server that adds fields later.
type hydraInfo struct {
	AppVersion       string `json:"appVersion"`
	Status           string `json:"status"`
	Product          string `json:"product"`
	Hostname         string `json:"hostname"`
	RemoteAPIVersion int    `json:"remoteApiVersion"`
}

// cmdStatus implements `hydra-cli status` - a real HTTP client call
// against a live (or unreachable) HYDRA-UMC-SERVER, not a stub. Target
// resolution (--server/--config/HYDRA_CLI_SERVER/default) is shared with
// `robots`/`doctor`/`shell` via resolveTarget (see server.go).
func cmdStatus(args []string) error {
	server, timeout, rest, err := resolveTarget(args)
	if err != nil {
		return err
	}
	if err := rejectExtraArgs("status", rest); err != nil {
		return err
	}

	client := http.Client{Timeout: timeout}
	resp, err := client.Get(server + "/api/hydra-info")
	if err != nil {
		return newCliError(ExitNetworkError, fmt.Errorf("could not reach %s: %w", server, err))
	}
	defer resp.Body.Close()

	body, err := readBody(resp.Body)
	if err != nil {
		return bodyReadError(server, "response", err)
	}

	if resp.StatusCode != http.StatusOK {
		return newCliError(ExitServerError, fmt.Errorf("%s replied with HTTP %d: %s", server, resp.StatusCode, printable(string(body))))
	}

	var info hydraInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return newCliError(ExitServerError, fmt.Errorf("unexpected response from %s: %w", server, err))
	}

	if strings.TrimSpace(info.AppVersion) == "" {
		return newCliError(ExitServerError, fmt.Errorf("%s answered /api/hydra-info without an appVersion: it does not look like a HYDRA-UMC-SERVER", server))
	}

	fmt.Printf("server:      %s\n", server)
	if info.Product != "" {
		fmt.Printf("product:     %s\n", printable(info.Product))
	}
	if info.Hostname != "" {
		fmt.Printf("hostname:    %s\n", printable(info.Hostname))
	}
	fmt.Printf("appVersion:  %s\n", printable(info.AppVersion))
	if info.RemoteAPIVersion != 0 {
		fmt.Printf("remoteApiVersion: %d\n", info.RemoteAPIVersion)
	}
	if info.Status != "" {
		fmt.Printf("status:      %s\n", printable(info.Status))
	}
	return nil
}
