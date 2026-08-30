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
	"io"
	"net/http"
	"os"
	"time"
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
    status [--server URL] Query a HYDRA-UMC-SERVER instance's /api/hydra-info
                          endpoint and print its reported identity/version.
    robots [--server URL] Query a HYDRA-UMC-SERVER instance's /api/settings
                          endpoint and print its real controller/robot roster.
                          Both default to %s; override with --server or the
                          HYDRA_CLI_SERVER environment variable.
	doctor [--server URL] Read-only diagnostic: validates /api/hydra-info and
	                      /api/settings, then verifies their controller/robot
	                      counts agree. It never sends commands or probes hardware.
    config validate --config PATH
                          Load and schema-validate a local config file.
    config apply --config PATH [--dry-run]
                          Preview (--dry-run) what a config apply would send.
                          Without --dry-run, exits ExitNotImplemented: no
                          live fleet-write endpoint exists yet.
    shell [--server URL]  Interactive REPL - run any command above
                          repeatedly against the same server without
                          restarting the process. exit/quit or Ctrl-D
                          to leave.
    help                  Show this message.

EXIT CODES:
    0  ok                     3  config error
    1  general error          4  network error (server unreachable)
    2  usage error            5  server error (bad status/response)
                              6  not implemented

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
// against a live (or unreachable) HYDRA-UMC-SERVER, not a stub. Server
// resolution (--server / HYDRA_CLI_SERVER / default) is shared with
// `robots` via resolveServer (see server.go).
func cmdStatus(args []string) error {
	server, _ := resolveServer(args)

	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(server + "/api/hydra-info")
	if err != nil {
		return newCliError(ExitNetworkError, fmt.Errorf("could not reach %s: %w", server, err))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return newCliError(ExitNetworkError, fmt.Errorf("reading response from %s: %w", server, err))
	}

	if resp.StatusCode != http.StatusOK {
		return newCliError(ExitServerError, fmt.Errorf("%s replied with HTTP %d: %s", server, resp.StatusCode, string(body)))
	}

	var info hydraInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return newCliError(ExitServerError, fmt.Errorf("unexpected response from %s: %w", server, err))
	}

	fmt.Printf("server:      %s\n", server)
	if info.Product != "" {
		fmt.Printf("product:     %s\n", info.Product)
	}
	if info.Hostname != "" {
		fmt.Printf("hostname:    %s\n", info.Hostname)
	}
	fmt.Printf("appVersion:  %s\n", info.AppVersion)
	if info.RemoteAPIVersion != 0 {
		fmt.Printf("remoteApiVersion: %d\n", info.RemoteAPIVersion)
	}
	if info.Status == "" {
		fmt.Println("status:      not reported")
	} else {
		fmt.Printf("status:      %s\n", info.Status)
	}
	return nil
}
