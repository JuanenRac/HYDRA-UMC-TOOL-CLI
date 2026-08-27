// =============================================================================
// HYDRA-UMC-TOOL-CLI - cmd/hydra-cli/server_test.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0 - see LICENSE
// =============================================================================
package main

import (
	"os"
	"reflect"
	"testing"
)

func TestResolveServer_DefaultsWhenNothingSet(t *testing.T) {
	os.Unsetenv("HYDRA_CLI_SERVER")
	server, rest := resolveServer(nil)
	if server != defaultServerURL {
		t.Fatalf("expected the default server URL, got %q", server)
	}
	if len(rest) != 0 {
		t.Fatalf("expected no remaining args, got %v", rest)
	}
}

func TestResolveServer_EnvVarOverridesDefault(t *testing.T) {
	t.Setenv("HYDRA_CLI_SERVER", "http://env-server:9000")
	server, _ := resolveServer(nil)
	if server != "http://env-server:9000" {
		t.Fatalf("expected the env var server URL, got %q", server)
	}
}

func TestResolveServer_FlagOverridesEnvVar(t *testing.T) {
	t.Setenv("HYDRA_CLI_SERVER", "http://env-server:9000")
	server, rest := resolveServer([]string{"--server", "http://flag-server:9001", "--other-flag"})
	if server != "http://flag-server:9001" {
		t.Fatalf("expected the --server flag to win over the env var, got %q", server)
	}
	if !reflect.DeepEqual(rest, []string{"--other-flag"}) {
		t.Fatalf("expected --server/its value stripped from remaining args, got %v", rest)
	}
}

func TestResolveServer_TrailingServerFlagWithNoValueIsIgnored(t *testing.T) {
	os.Unsetenv("HYDRA_CLI_SERVER")
	server, rest := resolveServer([]string{"--server"})
	if server != defaultServerURL {
		t.Fatalf("a --server flag with no following value should not change the resolved server, got %q", server)
	}
	if !reflect.DeepEqual(rest, []string{"--server"}) {
		t.Fatalf("a --server flag with no following value should be left in rest, got %v", rest)
	}
}
