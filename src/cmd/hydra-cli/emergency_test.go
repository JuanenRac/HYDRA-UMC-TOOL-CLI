// =============================================================================
// HYDRA-UMC-TOOL-CLI - cmd/hydra-cli/emergency_test.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0-or-later - see LICENSE
// =============================================================================
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

type recordedRequest struct {
	Method, Path, Controller, Auth string
	Body                           map[string]any
}

func recordingServer(t *testing.T, status int, reply string, record *atomic.Value, hits *int32) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(hits, 1)
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		record.Store(recordedRequest{Method: r.Method, Path: r.URL.Path, Controller: r.Header.Get("X-Controller-Id"), Auth: r.Header.Get("Authorization"), Body: body})
		w.WriteHeader(status)
		_, _ = w.Write([]byte(reply))
	}))
	t.Cleanup(server.Close)
	return server
}

func TestStop_SendsTheStopCommandForOneRobot(t *testing.T) {
	clearCredentials(t)
	t.Setenv(tokenEnv, "t")
	var seen atomic.Value
	var hits int32
	server := recordingServer(t, 200, `{"success":true,"revision":"abc123"}`, &seen, &hits)
	var out bytes.Buffer
	if err := cmdStop(&out, []string{"7", "--server", server.URL, "--controller-id", "bench"}); err != nil {
		t.Fatalf("cmdStop: %v", err)
	}
	got := seen.Load().(recordedRequest)
	if got.Method != "POST" || got.Path != "/api/robot/7/command" || got.Body["command"] != "stop" || got.Controller != "bench" || got.Auth != "Bearer t" {
		t.Fatalf("request = %+v", got)
	}
	if !strings.Contains(out.String(), "Stop sent to robot 7") || !strings.Contains(out.String(), "abc123") {
		t.Fatalf("output %q", out.String())
	}
}

func TestStop_Errors(t *testing.T) {
	clearCredentials(t)
	t.Setenv(tokenEnv, "t")
	var seen atomic.Value
	var hits int32
	server := recordingServer(t, 404, `{"error":"Robot not found"}`, &seen, &hits)
	if err := cmdStop(&bytes.Buffer{}, []string{"99", "--server", server.URL}); exitCodeFor(err) != ExitServerError {
		t.Fatalf("unknown robot: %v", err)
	}
	for _, args := range [][]string{{}, {"x"}, {"-1"}, {"1", "2"}} {
		if err := cmdStop(&bytes.Buffer{}, append(args, "--server", server.URL)); exitCodeFor(err) != ExitUsageError {
			t.Errorf("stop %v: %v", args, err)
		}
	}
}

func TestStopAll_WithoutYesSendsNothing(t *testing.T) {
	clearCredentials(t)
	t.Setenv(tokenEnv, "t")
	var seen atomic.Value
	var hits int32
	server := recordingServer(t, 200, `{}`, &seen, &hits)
	err := cmdStopAll(&bytes.Buffer{}, []string{"--server", server.URL})
	if exitCodeFor(err) != ExitUsageError || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("err = %v", err)
	}
	if atomic.LoadInt32(&hits) != 0 {
		t.Fatal("the global stop was sent without a confirmation")
	}
}

func TestStopAll_ReportsEachRobotAndItsReservation(t *testing.T) {
	clearCredentials(t)
	t.Setenv(tokenEnv, "t")
	var seen atomic.Value
	var hits int32
	reply := `{"success":true,"stoppedCount":2,"failedCount":0,"persisted":true,"robots":[` +
		`{"robotId":1,"name":"A1","stopped":true,"valvesOff":2,"pumpsOff":1},` +
		`{"robotId":2,"name":"A2","stopped":true,"valvesOff":0,"pumpsOff":0,"reservedBy":"maria"}]}`
	server := recordingServer(t, 200, reply, &seen, &hits)
	var out bytes.Buffer
	if err := cmdStopAll(&out, []string{"--yes", "--server", server.URL}); err != nil {
		t.Fatalf("cmdStopAll: %v", err)
	}
	if got := seen.Load().(recordedRequest); got.Method != "POST" || got.Path != "/api/stop-all" {
		t.Fatalf("request = %+v", got)
	}
	for _, want := range []string{"2 robot(s) stopped, 0 failed", "A1", "A2", "maria"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
}

func TestStopAll_AFailureIsAServerErrorAndSaysSo(t *testing.T) {
	clearCredentials(t)
	t.Setenv(tokenEnv, "t")
	var seen atomic.Value
	var hits int32
	reply := `{"success":false,"stoppedCount":1,"failedCount":1,"persisted":false,"robots":[` +
		`{"robotId":1,"name":"A1","stopped":true},{"robotId":2,"name":"A2","stopped":false,"error":"disk\nfull"}]}`
	server := recordingServer(t, 200, reply, &seen, &hits)
	var out bytes.Buffer
	err := cmdStopAll(&out, []string{"--yes", "--server", server.URL})
	if exitCodeFor(err) != ExitServerError || !strings.Contains(err.Error(), "1 robot(s) could not be stopped") {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(out.String(), "NO disk?full") || !strings.Contains(out.String(), "could not be saved") {
		t.Fatalf("the failure and the unsaved settings must be on screen, with control characters removed:\n%s", out.String())
	}
}

func TestStopAll_JSONPassesTheServerAnswerThrough(t *testing.T) {
	clearCredentials(t)
	t.Setenv(tokenEnv, "t")
	var seen atomic.Value
	var hits int32
	server := recordingServer(t, 200, `{"success":true,"stoppedCount":0,"failedCount":0,"persisted":true,"robots":[]}`, &seen, &hits)
	var out bytes.Buffer
	if err := cmdStopAll(&out, []string{"--yes", "--json", "--server", server.URL}); err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if json.Unmarshal(out.Bytes(), &parsed) != nil || parsed["stoppedCount"] != float64(0) {
		t.Fatalf("not the server's JSON: %q", out.String())
	}
}

// ---- metrics ----------------------------------------------------------------

const samplePage = `# HELP hydra_process_uptime_seconds Uptime
# TYPE hydra_process_uptime_seconds gauge
hydra_process_uptime_seconds 123.5
hydra_ws_clients_connected 3
hydra_robot_commands_total{command="jog"} 41
hydra_robot_commands_total{command="stop"} 2
hydra_auth_failures_total{reason="invalid_credentials"} 5
hydra_system_temp_celsius 51.2
some_other_metric{note="a \"quoted\" value, with a comma",x="1"} 7e2
`

func TestParseMetrics_ReadsLabelsQuotesAndExponents(t *testing.T) {
	samples, err := parseMetrics(samplePage)
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 7 {
		t.Fatalf("%d samples: %+v", len(samples), samples)
	}
	last := samples[6]
	if last.Name != "some_other_metric" || last.Value != 700 || last.Labels["note"] != `a "quoted" value, with a comma` || last.Labels["x"] != "1" {
		t.Fatalf("last = %+v", last)
	}
	if samples[2].Labels["command"] != "jog" || samples[2].Value != 41 {
		t.Fatalf("labelled sample = %+v", samples[2])
	}
	for _, bad := range []string{"just_a_name", "name{a=\"1\" 5", "name{a=1} 5", "name abc", "{a=\"1\"} 5"} {
		if _, err := parseMetrics(bad); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
}

func TestMetrics_SummaryAllAndJSON(t *testing.T) {
	clearCredentials(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/metrics" {
			w.WriteHeader(404)
			return
		}
		_, _ = w.Write([]byte(samplePage))
	}))
	defer server.Close()

	var summary bytes.Buffer
	if err := cmdMetrics(&summary, []string{"--server", server.URL}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"uptime (s)", "123.5", "WebSocket clients", "commands sent", "command=jog", "41", "login and token failures", "reason=invalid_credentials", "temperature"} {
		if !strings.Contains(summary.String(), want) {
			t.Errorf("summary lacks %q:\n%s", want, summary.String())
		}
	}
	if strings.Contains(summary.String(), "some_other_metric") {
		t.Errorf("the summary must show only the usual figures:\n%s", summary.String())
	}
	var all bytes.Buffer
	if err := cmdMetrics(&all, []string{"--server", server.URL, "--all"}); err != nil || !strings.Contains(all.String(), "some_other_metric") {
		t.Fatalf("--all: %v\n%s", err, all.String())
	}
	var asJSON bytes.Buffer
	if err := cmdMetrics(&asJSON, []string{"--server", server.URL, "--json"}); err != nil {
		t.Fatal(err)
	}
	var samples []sample
	if json.Unmarshal(asJSON.Bytes(), &samples) != nil || len(samples) != 7 {
		t.Fatalf("--json: %q", asJSON.String())
	}
}

func TestMetrics_AuthAndErrors(t *testing.T) {
	clearCredentials(t)
	guarded := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer ok" {
			w.WriteHeader(401)
			return
		}
		_, _ = w.Write([]byte(samplePage))
	}))
	defer guarded.Close()
	if err := cmdMetrics(&bytes.Buffer{}, []string{"--server", guarded.URL}); exitCodeFor(err) != ExitAuthError {
		t.Fatalf("no credential: %v", err)
	}
	t.Setenv(tokenEnv, "ok")
	if err := cmdMetrics(&bytes.Buffer{}, []string{"--server", guarded.URL}); err != nil {
		t.Fatalf("with the credential: %v", err)
	}
	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("not a metric line")) }))
	defer broken.Close()
	if err := cmdMetrics(&bytes.Buffer{}, []string{"--server", broken.URL}); exitCodeFor(err) != ExitServerError {
		t.Fatalf("garbage: %v", err)
	}
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer failing.Close()
	if err := cmdMetrics(&bytes.Buffer{}, []string{"--server", failing.URL}); exitCodeFor(err) != ExitServerError {
		t.Fatalf("503: %v", err)
	}
	if err := cmdMetrics(&bytes.Buffer{}, []string{"--server", failing.URL, "extra"}); exitCodeFor(err) != ExitUsageError {
		t.Fatalf("extra argument: %v", err)
	}
}
