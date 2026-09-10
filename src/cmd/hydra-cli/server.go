// =============================================================================
// HYDRA-UMC-TOOL-CLI - Shared HYDRA-UMC-SERVER target resolution: cmd/hydra-cli/server.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0 - see LICENSE
// =============================================================================
// One real implementation of "which HYDRA-UMC-SERVER instance is this
// invocation talking to, and with what timeout", shared by every
// subcommand that needs it (status, robots, doctor, shell) rather than
// each reimplementing the same --server/--config/HYDRA_CLI_SERVER/default
// precedence and risking it drifting out of sync between commands.
package main

import (
	"os"
	"strconv"
	"time"
)

// defaultRequestTimeout is resolveTarget's own fallback request timeout,
// used when neither a --config file's "timeoutSec" nor a
// HYDRA_CLI_TIMEOUT_SEC session default (see shell.go) applies.
const defaultRequestTimeout = 5 * time.Second

// resolveTarget picks the target HYDRA-UMC-SERVER base URL AND the
// request timeout every live command (status/robots/doctor/shell) uses -
// found while auditing the code: `config
// validate`/`config apply` already schema-validate a real local config
// file (config.go), but no live command ever consulted it, so --config
// only ever affected validation, never a real run. Precedence for the
// server: an explicit --server flag wins, then a --config file's own
// "server", then the HYDRA_CLI_SERVER environment variable (set for the
// rest of a `shell` session - see shell.go - or by an operator's own
// shell), then defaultServerURL. Precedence for the timeout: a --config
// file's own "timeoutSec", then HYDRA_CLI_TIMEOUT_SEC (the same session
// mechanism), then defaultRequestTimeout - there is no standalone
// --timeout flag today, only a config file sets one explicitly.
// Returns args with --server/--config and their values removed, so
// callers keep parsing their own remaining flags unaware of either. A
// malformed/invalid --config file surfaces as the same real
// ExitConfigError loadConfig already produces, never silently ignored.
func resolveTarget(args []string) (server string, timeout time.Duration, rest []string, err error) {
	server = os.Getenv("HYDRA_CLI_SERVER")
	if server == "" {
		server = defaultServerURL
	}
	timeout = defaultRequestTimeout
	if envTimeout, ok := parsePositiveSeconds(os.Getenv("HYDRA_CLI_TIMEOUT_SEC")); ok {
		timeout = envTimeout
	}

	var explicitServer, configPath string
	rest = make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if args[i] == "--server" && i+1 < len(args) {
			explicitServer = args[i+1]
			i++
			continue
		}
		if args[i] == "--config" && i+1 < len(args) {
			configPath = args[i+1]
			i++
			continue
		}
		rest = append(rest, args[i])
	}

	if configPath != "" {
		cfg, loadErr := loadConfig(configPath)
		if loadErr != nil {
			return "", 0, nil, loadErr
		}
		server = cfg.Server
		timeout = time.Duration(cfg.TimeoutSec) * time.Second
	}

	if explicitServer != "" {
		server = explicitServer
	}

	return server, timeout, rest, nil
}

// parsePositiveSeconds parses a HYDRA_CLI_TIMEOUT_SEC-style value, only
// ever returning ok=true for a positive integer - an unset, empty,
// malformed, zero, or negative value is treated exactly like "not set"
// rather than producing a zero or negative time.Duration a real
// http.Client would treat as "no timeout at all".
func parsePositiveSeconds(raw string) (time.Duration, bool) {
	if raw == "" {
		return 0, false
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0, false
	}
	return time.Duration(n) * time.Second, true
}
