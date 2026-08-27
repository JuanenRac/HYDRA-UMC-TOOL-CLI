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
