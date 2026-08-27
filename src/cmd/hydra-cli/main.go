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
	args := os.Args[1:]

	if len(args) == 0 {
		printHelp()
		os.Exit(0)
	}

	switch args[0] {
	case "version", "-v", "--version":
		cmdVersion()
	case "help", "-h", "--help":
		printHelp()
	case "status":
		if err := cmdStatus(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "hydra-cli status: %v\n", err)
			os.Exit(1)
		}
	case "robots":
		if err := cmdRobots(os.Stdout, args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "hydra-cli robots: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "hydra-cli: unknown command %q\n\n", args[0])
		printHelp()
		os.Exit(1)
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
    help                  Show this message.

Report issues at: https://github.com/JuanenRac/HYDRA-UMC-TOOL-CLI
`, projectName, Version, defaultServerURL)
}

// hydraInfo mirrors the subset of HYDRA-UMC-SERVER's GET /api/hydra-info
// response this CLI actually reads. Unknown fields are ignored, so this
// stays forward-compatible with a server that adds fields later.
type hydraInfo struct {
	AppVersion string `json:"appVersion"`
	Status     string `json:"status"`
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
		return fmt.Errorf("could not reach %s: %w", server, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response from %s: %w", server, err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s replied with HTTP %d: %s", server, resp.StatusCode, string(body))
	}

	var info hydraInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return fmt.Errorf("unexpected response from %s: %w", server, err)
	}

	fmt.Printf("server:      %s\n", server)
	fmt.Printf("appVersion:  %s\n", info.AppVersion)
	fmt.Printf("status:      %s\n", info.Status)
	return nil
}
