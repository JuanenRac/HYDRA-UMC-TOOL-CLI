// =============================================================================
// HYDRA-UMC-TOOL-CLI - Shared HYDRA-UMC-SERVER target resolution: cmd/hydra-cli/server.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0 - see LICENSE
// =============================================================================
// One real implementation of "which HYDRA-UMC-SERVER instance is this
// invocation talking to", shared by every subcommand that needs it
// (status, robots, ...) rather than each reimplementing the same
// --server/HYDRA_CLI_SERVER/default precedence and risking it drifting
// out of sync between commands.
package main

import "os"

// resolveServer picks the target HYDRA-UMC-SERVER base URL: an explicit
// --server flag wins, then the HYDRA_CLI_SERVER environment variable,
// then defaultServerURL. Returns the resolved server URL plus args with
// any --server <url> pair removed, so callers can keep parsing their own
// remaining flags without seeing it.
func resolveServer(args []string) (server string, rest []string) {
	server = os.Getenv("HYDRA_CLI_SERVER")
	if server == "" {
		server = defaultServerURL
	}

	rest = make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if args[i] == "--server" && i+1 < len(args) {
			server = args[i+1]
			i++
			continue
		}
		rest = append(rest, args[i])
	}
	return server, rest
}
