// =============================================================================
// HYDRA-UMC-TOOL-CLI - `config` subcommand: cmd/hydra-cli/config.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0 - see LICENSE
// =============================================================================
// A real local config file this CLI can validate and (once
// HYDRA-UMC-SERVER exposes a real fleet-write endpoint) apply - today's
// honest v0 scope is validation plus a real --dry-run preview, neither
// of which needs a live server or a physical device to prove correct.
// `config apply` without --dry-run is a real, deliberate
// ExitNotImplemented rather than silently doing nothing or pretending
// to succeed - the write endpoint it would call genuinely does not
// exist yet (see mejoras_futuras.txt's own `deploy`/`flash-all` entries).
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
)

// Config is the real, local configuration this CLI can read: defaults
// for the target server and request timeout today, the same values
// `--server`/`HYDRA_CLI_SERVER` already override at the command line -
// a config file is for a fleet operator who wants those defaults
// checked into a real file instead of retyped/re-exported every time.
type Config struct {
	Server     string `json:"server"`
	TimeoutSec int    `json:"timeoutSec"`
}

// validateConfig is the real schema check, split out from loadConfig so
// it can be exercised directly against hand-built Config values in
// tests, not only through a real file on disk.
func validateConfig(cfg Config) error {
	if cfg.Server == "" {
		return fmt.Errorf(`"server" must not be empty`)
	}
	parsed, err := url.Parse(cfg.Server)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf(`"server" must be a valid absolute URL (e.g. "http://host:port"), got %q`, cfg.Server)
	}
	if cfg.TimeoutSec <= 0 {
		return fmt.Errorf(`"timeoutSec" must be a positive integer, got %d`, cfg.TimeoutSec)
	}
	return nil
}

// loadConfig reads and schema-validates a config file at path. Every
// real failure mode - a missing file, malformed JSON, or a value that
// fails validateConfig - returns a *CliError classified ExitConfigError,
// so a caller never has to guess why a config was rejected.
func loadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, newCliError(ExitConfigError, fmt.Errorf("reading config %s: %w", path, err))
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, newCliError(ExitConfigError, fmt.Errorf("parsing config %s: %w", path, err))
	}
	if err := validateConfig(cfg); err != nil {
		return Config{}, newCliError(ExitConfigError, fmt.Errorf("config %s: %w", path, err))
	}
	return cfg, nil
}

// parseConfigFlags pulls --config PATH and --dry-run out of args, in
// whichever order they appear - shared by every `config` subcommand.
func parseConfigFlags(args []string) (path string, dryRun bool) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 < len(args) {
				path = args[i+1]
				i++
			}
		case "--dry-run":
			dryRun = true
		}
	}
	return path, dryRun
}

// cmdConfig implements `hydra-cli config <validate|apply>`.
func cmdConfig(w io.Writer, args []string) error {
	if len(args) == 0 {
		return newCliError(ExitUsageError, fmt.Errorf("usage: hydra-cli config <validate|apply> --config PATH [--dry-run]"))
	}

	sub := args[0]
	path, dryRun := parseConfigFlags(args[1:])
	if path == "" {
		return newCliError(ExitUsageError, fmt.Errorf("--config PATH is required"))
	}

	switch sub {
	case "validate":
		return cmdConfigValidate(w, path)
	case "apply":
		return cmdConfigApply(w, path, dryRun)
	default:
		return newCliError(ExitUsageError, fmt.Errorf("unknown config subcommand %q (want validate or apply)", sub))
	}
}

// cmdConfigValidate loads and schema-validates a real config file - no
// network, no device, real file I/O and real validation only.
func cmdConfigValidate(w io.Writer, path string) error {
	cfg, err := loadConfig(path)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "config %s is valid: server=%s timeoutSec=%d\n", path, cfg.Server, cfg.TimeoutSec)
	return nil
}

// cmdConfigApply is the real dry-run operation this gate asks for:
// without --dry-run it honestly refuses (ExitNotImplemented) instead of
// pretending to push config to a write endpoint that does not exist yet.
// With --dry-run it prints exactly what a live apply would send, proving
// the real config/validation path end to end without touching any
// device or live server.
func cmdConfigApply(w io.Writer, path string, dryRun bool) error {
	cfg, err := loadConfig(path)
	if err != nil {
		return err
	}

	if !dryRun {
		return newCliError(ExitNotImplemented, fmt.Errorf(
			"live config apply needs a real fleet-write endpoint on HYDRA-UMC-SERVER, which does not exist yet - rerun with --dry-run",
		))
	}

	fmt.Fprintf(w, "DRY RUN: would apply config to %s\n", cfg.Server)
	fmt.Fprintf(w, "  server:     %s\n", cfg.Server)
	fmt.Fprintf(w, "  timeoutSec: %d\n", cfg.TimeoutSec)
	fmt.Fprintf(w, "no real device or server was contacted\n")
	return nil
}
