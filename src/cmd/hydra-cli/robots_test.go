// =============================================================================
// HYDRA-UMC-TOOL-CLI - cmd/hydra-cli/robots_test.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0 - see LICENSE
// =============================================================================
package main

import (
	"bytes"
	"encoding/json"
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

// TestCmdRobots_ConfigFileDrivesRealServerTarget closes the same real gap
// documented in status_test.go's own TestCmdStatus_ConfigFileDrivesRealServerTarget:
// --config alone (no --server) must really drive this command's HTTP
// target, not just be schema-validated and discarded.
func TestCmdRobots_ConfigFileDrivesRealServerTarget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"controllers": [{"name": "C1", "robots": [{"name": "R1", "online": true}]}]}`))
	}))
	defer server.Close()

	path := writeConfigFile(t, `{"server": "`+server.URL+`", "timeoutSec": 5}`)
	var stdout bytes.Buffer
	if err := cmdRobots(&stdout, []string{"--config", path}); err != nil {
		t.Fatalf("cmdRobots with only --config returned an error against a real, healthy server: %v", err)
	}
	if !strings.Contains(stdout.String(), "R1") {
		t.Fatalf("expected the real robot from the config-resolved server, got:\n%s", stdout.String())
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

const rosterBody = `{"controllers": [
	{"name": "Line 1", "robots": [
		{"name": "A", "online": true, "model": "Parol6", "role": "Pnp"},
		{"name": "Long robot name", "online": false, "model": "X", "role": "Assembly"}]}]}`

func rosterServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
}

func TestCmdRobots_JSONIsAnArrayOfRows(t *testing.T) {
	server := rosterServer(t, rosterBody)
	defer server.Close()

	var out bytes.Buffer
	if err := cmdRobots(&out, []string{"--server", server.URL, "--json"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var rows []robotRow
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	if len(rows) != 2 || rows[0].Robot != "A" || !rows[0].Online || rows[1].Controller != "Line 1" || rows[1].Online {
		t.Fatalf("unexpected rows: %+v", rows)
	}
}

func TestCmdRobots_JSONWithNoRobotsIsAnEmptyArray(t *testing.T) {
	server := rosterServer(t, `{"controllers": []}`)
	defer server.Close()

	var out bytes.Buffer
	if err := cmdRobots(&out, []string{"--json", "--server", server.URL}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(out.String()) != "[]" {
		t.Fatalf("want [], got %q", out.String())
	}
}

func TestCmdRobots_TableColumnsLineUpWhateverTheNameLength(t *testing.T) {
	server := rosterServer(t, rosterBody)
	defer server.Close()

	var out bytes.Buffer
	if err := cmdRobots(&out, []string{"--server", server.URL}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	column := -1
	for _, line := range strings.Split(strings.TrimRight(out.String(), "\n"), "\n") {
		at := strings.Index(line, "no ")
		if at < 0 {
			at = strings.Index(line, "yes")
		}
		if at < 0 {
			at = strings.Index(line, "ONLINE")
		}
		if column == -1 {
			column = at
		}
		if at != column {
			t.Fatalf("the ONLINE column moves (%d vs %d):\n%s", at, column, out.String())
		}
	}
}

func TestCmdRobots_UnknownArgumentStillRefused(t *testing.T) {
	var out bytes.Buffer
	if err := cmdRobots(&out, []string{"--server", "http://127.0.0.1:1", "--bogus"}); err == nil || exitCodeFor(err) != ExitUsageError {
		t.Fatalf("want a usage error, got %v", err)
	}
}
