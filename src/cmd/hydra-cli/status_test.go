// =============================================================================
// HYDRA-UMC-TOOL-CLI - cmd/hydra-cli/status_test.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0 - see LICENSE
// =============================================================================
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCmdStatus_RealHTTPRoundTrip(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/hydra-info" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"appVersion": "0.1.5", "status": "running"}`))
	}))
	defer server.Close()

	if err := cmdStatus([]string{"--server", server.URL}); err != nil {
		t.Fatalf("cmdStatus returned an error against a real, healthy server: %v", err)
	}
}

func TestCmdStatus_ReturnsErrorOnUnreachableServer(t *testing.T) {
	err := cmdStatus([]string{"--server", "http://127.0.0.1:1"})
	if err == nil {
		t.Fatal("expected a real error for an unreachable server, got nil")
	}
	if !strings.Contains(err.Error(), "could not reach") {
		t.Fatalf("expected a real reachability error, got: %v", err)
	}
}

// TestCmdStatus_ConfigFileDrivesRealServerTarget closes the real gap found
// in an ecosystem-wide software-improvements audit: a --config file was
// already schema-validated by `config validate`/`config apply`, but no
// live command ever actually consulted it - status still only looked at
// --server/HYDRA_CLI_SERVER/the compiled-in default. Passing --config
// alone (no --server at all) here proves the config file's own "server"
// really drives this real HTTP round trip now.
func TestCmdStatus_ConfigFileDrivesRealServerTarget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"appVersion": "0.1.5", "status": "running"}`))
	}))
	defer server.Close()

	path := writeConfigFile(t, `{"server": "`+server.URL+`", "timeoutSec": 5}`)
	if err := cmdStatus([]string{"--config", path}); err != nil {
		t.Fatalf("cmdStatus with only --config returned an error against a real, healthy server: %v", err)
	}
}

// TestCmdStatus_ConfigFileTimeoutIsEnforced proves the other half of the
// same gap: a --config file's own "timeoutSec" now really becomes the
// http.Client's real request timeout, not just a schema-validated number
// nothing downstream ever reads.
func TestCmdStatus_ConfigFileTimeoutIsEnforced(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"appVersion": "0.1.5"}`))
	}))
	defer server.Close()

	path := writeConfigFile(t, `{"server": "`+server.URL+`", "timeoutSec": 1}`)
	start := time.Now()
	err := cmdStatus([]string{"--config", path})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected the real 1s config timeout to fire against a handler that sleeps 1.2s")
	}
	assertCliErrorCode(t, err, ExitNetworkError)
	if elapsed >= 1200*time.Millisecond {
		t.Fatalf("expected the request to time out around 1s, not wait for the full 1.2s handler; took %v", elapsed)
	}
}

func TestCmdStatus_ReturnsErrorOnMalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	if err := cmdStatus([]string{"--server", server.URL}); err == nil {
		t.Fatal("expected a real error for a malformed response, got nil")
	}
}
