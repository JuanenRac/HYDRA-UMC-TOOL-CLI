// =============================================================================
// HYDRA-UMC-TOOL-CLI - cmd/hydra-cli/robots_test.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0 - see LICENSE
// =============================================================================
package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCmdRobots_RealHTTPRoundTrip(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/settings" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"controllers": [
				{
					"id": 1, "name": "Controller A", "ip": "192.168.1.10", "status": "online",
					"robots": [
						{"id": 1, "name": "Robot A1", "online": true, "model": "Parol6 (6-DOF)", "role": "Pnp"},
						{"id": 2, "name": "Robot A2", "online": false, "model": "Parol6 (6-DOF)", "role": "Assembly"}
					]
				}
			]
		}`))
	}))
	defer server.Close()

	var stdout bytes.Buffer
	if err := cmdRobots(&stdout, []string{"--server", server.URL}); err != nil {
		t.Fatalf("cmdRobots returned an error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Robot A1") || !strings.Contains(output, "Robot A2") {
		t.Fatalf("expected both real robots in output, got:\n%s", output)
	}
	if !strings.Contains(output, "Controller A") {
		t.Fatalf("expected the real controller name in output, got:\n%s", output)
	}
}

func TestCmdRobots_ReturnsErrorOnUnreachableServer(t *testing.T) {
	// Port 1 is a real, universally-unassigned low port nothing binds to
	// in this test environment - a real connection refusal, not simulated.
	var discard bytes.Buffer
	err := cmdRobots(&discard, []string{"--server", "http://127.0.0.1:1"})
	if err == nil {
		t.Fatal("expected a real error for an unreachable server, got nil")
	}
}

func TestCmdRobots_ReturnsErrorOnHTTPFailureStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer server.Close()

	var discard bytes.Buffer
	err := cmdRobots(&discard, []string{"--server", server.URL})
	if err == nil {
		t.Fatal("expected a real error for a 500 response, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("expected the real status code in the error, got: %v", err)
	}
}

func TestPrintRobotRoster_HandlesEmptyRosterHonestly(t *testing.T) {
	var buf bytes.Buffer
	printRobotRoster(&buf, "http://example.test", settingsResponse{})
	if !strings.Contains(buf.String(), "no robots reported") {
		t.Fatalf("expected an honest empty-roster message, got: %s", buf.String())
	}
}

func TestPrintRobotRoster_PrintsOnlineStatusCorrectly(t *testing.T) {
	var buf bytes.Buffer
	printRobotRoster(&buf, "http://example.test", settingsResponse{
		Controllers: []controllerEntry{
			{Name: "C1", Robots: []robotEntry{
				{Name: "Online-Robot", Online: true, Model: "M1", Role: "R1"},
				{Name: "Offline-Robot", Online: false, Model: "M2", Role: "R2"},
			}},
		},
	})
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 { // header + 2 robots
		t.Fatalf("expected 3 lines (header + 2 robots), got %d:\n%s", len(lines), buf.String())
	}
	if !strings.Contains(lines[1], "yes") {
		t.Fatalf("expected Online-Robot's row to say yes, got: %s", lines[1])
	}
	if !strings.Contains(lines[2], "no") {
		t.Fatalf("expected Offline-Robot's row to say no, got: %s", lines[2])
	}
}
