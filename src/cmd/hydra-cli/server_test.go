// =============================================================================
// HYDRA-UMC-TOOL-CLI - cmd/hydra-cli/server_test.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0 - see LICENSE
// =============================================================================
package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestResolveTarget_DefaultsWhenNothingSet(t *testing.T) {
	os.Unsetenv("HYDRA_CLI_SERVER")
	os.Unsetenv("HYDRA_CLI_TIMEOUT_SEC")
	server, timeout, rest, err := resolveTarget(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if server != defaultServerURL {
		t.Fatalf("expected the default server URL, got %q", server)
	}
	if timeout != defaultRequestTimeout {
		t.Fatalf("expected the default request timeout, got %v", timeout)
	}
	if len(rest) != 0 {
		t.Fatalf("expected no remaining args, got %v", rest)
	}
}

func TestResolveTarget_EnvVarOverridesDefault(t *testing.T) {
	t.Setenv("HYDRA_CLI_SERVER", "http://env-server:9000")
	server, _, _, err := resolveTarget(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if server != "http://env-server:9000" {
		t.Fatalf("expected the env var server URL, got %q", server)
	}
}

func TestResolveTarget_TimeoutEnvVarOverridesDefault(t *testing.T) {
	os.Unsetenv("HYDRA_CLI_SERVER")
	t.Setenv("HYDRA_CLI_TIMEOUT_SEC", "12")
	_, timeout, _, err := resolveTarget(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if timeout != 12*time.Second {
		t.Fatalf("expected the env var timeout, got %v", timeout)
	}
}

func TestResolveTarget_MalformedTimeoutEnvVarIsIgnored(t *testing.T) {
	os.Unsetenv("HYDRA_CLI_SERVER")
	for _, malformed := range []string{"not-a-number", "0", "-5"} {
		t.Run(malformed, func(t *testing.T) {
			t.Setenv("HYDRA_CLI_TIMEOUT_SEC", malformed)
			_, timeout, _, err := resolveTarget(nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if timeout != defaultRequestTimeout {
				t.Fatalf("expected a malformed/non-positive timeout env var to fall back to the default, got %v", timeout)
			}
		})
	}
}

func TestResolveTarget_FlagOverridesEnvVar(t *testing.T) {
	t.Setenv("HYDRA_CLI_SERVER", "http://env-server:9000")
	server, _, rest, err := resolveTarget([]string{"--server", "http://flag-server:9001", "--other-flag"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if server != "http://flag-server:9001" {
		t.Fatalf("expected the --server flag to win over the env var, got %q", server)
	}
	if !reflect.DeepEqual(rest, []string{"--other-flag"}) {
		t.Fatalf("expected --server/its value stripped from remaining args, got %v", rest)
	}
}

func TestResolveTarget_TrailingServerFlagWithNoValueIsIgnored(t *testing.T) {
	os.Unsetenv("HYDRA_CLI_SERVER")
	server, _, rest, err := resolveTarget([]string{"--server"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if server != defaultServerURL {
		t.Fatalf("a --server flag with no following value should not change the resolved server, got %q", server)
	}
	if !reflect.DeepEqual(rest, []string{"--server"}) {
		t.Fatalf("a --server flag with no following value should be left in rest, got %v", rest)
	}
}

func TestResolveTarget_TrailingConfigFlagWithNoValueIsIgnored(t *testing.T) {
	os.Unsetenv("HYDRA_CLI_SERVER")
	server, timeout, rest, err := resolveTarget([]string{"--config"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if server != defaultServerURL || timeout != defaultRequestTimeout {
		t.Fatalf("a --config flag with no following value should not change the resolved target, got server=%q timeout=%v", server, timeout)
	}
	if !reflect.DeepEqual(rest, []string{"--config"}) {
		t.Fatalf("a --config flag with no following value should be left in rest, got %v", rest)
	}
}

func TestResolveTarget_ConfigFileSetsServerAndTimeout(t *testing.T) {
	os.Unsetenv("HYDRA_CLI_SERVER")
	os.Unsetenv("HYDRA_CLI_TIMEOUT_SEC")
	path := writeConfigFile(t, `{"server": "http://fleet-config-server:7000", "timeoutSec": 15}`)

	server, timeout, rest, err := resolveTarget([]string{"--config", path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if server != "http://fleet-config-server:7000" {
		t.Fatalf("expected the config file's own server, got %q", server)
	}
	if timeout != 15*time.Second {
		t.Fatalf("expected the config file's own timeoutSec, got %v", timeout)
	}
	if len(rest) != 0 {
		t.Fatalf("expected --config/its value stripped from remaining args, got %v", rest)
	}
}

func TestResolveTarget_ExplicitServerFlagWinsOverConfigFile(t *testing.T) {
	os.Unsetenv("HYDRA_CLI_SERVER")
	path := writeConfigFile(t, `{"server": "http://fleet-config-server:7000", "timeoutSec": 15}`)

	server, timeout, _, err := resolveTarget([]string{"--config", path, "--server", "http://override:9002"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if server != "http://override:9002" {
		t.Fatalf("expected the explicit --server flag to win over the config file, got %q", server)
	}
	// --server only overrides the server, not the timeout the config file
	// already validated - there is no standalone --timeout flag to compete
	// with it.
	if timeout != 15*time.Second {
		t.Fatalf("expected the config file's own timeoutSec to still apply, got %v", timeout)
	}
}

func TestResolveTarget_ConfigFileWinsOverEnvVar(t *testing.T) {
	t.Setenv("HYDRA_CLI_SERVER", "http://env-server:9000")
	t.Setenv("HYDRA_CLI_TIMEOUT_SEC", "1")
	path := writeConfigFile(t, `{"server": "http://fleet-config-server:7000", "timeoutSec": 15}`)

	server, timeout, _, err := resolveTarget([]string{"--config", path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if server != "http://fleet-config-server:7000" {
		t.Fatalf("expected the --config file to win over HYDRA_CLI_SERVER, got %q", server)
	}
	if timeout != 15*time.Second {
		t.Fatalf("expected the --config file to win over HYDRA_CLI_TIMEOUT_SEC, got %v", timeout)
	}
}

func TestResolveTarget_MissingConfigFileIsARealConfigError(t *testing.T) {
	_, _, _, err := resolveTarget([]string{"--config", filepath.Join(t.TempDir(), "does-not-exist.json")})
	assertCliErrorCode(t, err, ExitConfigError)
}

func TestResolveTarget_InvalidConfigFileIsARealConfigError(t *testing.T) {
	path := writeConfigFile(t, `{"server": "", "timeoutSec": 5}`)
	_, _, _, err := resolveTarget([]string{"--config", path})
	assertCliErrorCode(t, err, ExitConfigError)
}
