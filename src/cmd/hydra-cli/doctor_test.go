// =============================================================================
// HYDRA-UMC-TOOL-CLI - Read-only diagnostic tests: cmd/hydra-cli/doctor_test.go
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

func TestCmdDoctor_HealthyContractsPass(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/hydra-info":
			_, _ = w.Write([]byte(`{"schema_version":"1.0","appVersion":"0.2.4","remoteApiVersion":2,"controllerCount":1,"robotCount":2}`))
		case "/api/settings":
			_, _ = w.Write([]byte(`{"controllers":[{"name":"Master","robots":[{"name":"A1"},{"name":"A2"}]}]}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	var output bytes.Buffer
	if err := cmdDoctor(&output, []string{"--server", server.URL}); err != nil {
		t.Fatalf("cmdDoctor returned an error: %v", err)
	}
	if !strings.Contains(output.String(), "DOCTOR=PASS") || !strings.Contains(output.String(), "countCrossCheck=pass") {
		t.Fatalf("unexpected diagnostic output: %q", output.String())
	}
}

// TestCmdDoctor_ConfigFileDrivesRealServerTarget closes the same real gap
// documented in status_test.go's own TestCmdStatus_ConfigFileDrivesRealServerTarget:
// --config alone (no --server) must really drive both of doctor's own
// requests, not just be schema-validated and discarded.
func TestCmdDoctor_ConfigFileDrivesRealServerTarget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/hydra-info":
			_, _ = w.Write([]byte(`{"appVersion":"0.2.4","controllerCount":1,"robotCount":1}`))
		case "/api/settings":
			_, _ = w.Write([]byte(`{"controllers":[{"name":"C1","robots":[{"name":"R1"}]}]}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	path := writeConfigFile(t, `{"server": "`+server.URL+`", "timeoutSec": 5}`)
	var output bytes.Buffer
	if err := cmdDoctor(&output, []string{"--config", path}); err != nil {
		t.Fatalf("cmdDoctor with only --config returned an error against a real, healthy server: %v", err)
	}
	if !strings.Contains(output.String(), "DOCTOR=PASS") {
		t.Fatalf("expected a passing diagnosis against the config-resolved server, got: %q", output.String())
	}
}

func TestCmdDoctor_RejectsPublishedCountMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/hydra-info" {
			_, _ = w.Write([]byte(`{"appVersion":"0.2.4","controllerCount":1,"robotCount":8}`))
			return
		}
		_, _ = w.Write([]byte(`{"controllers":[{"robots":[{"name":"A1"}]}]}`))
	}))
	defer server.Close()

	var discard bytes.Buffer
	err := cmdDoctor(&discard, []string{"--server", server.URL})
	assertCliErrorCode(t, err, ExitServerError)
	if !strings.Contains(err.Error(), "robot count mismatch") {
		t.Fatalf("expected a count mismatch error, got: %v", err)
	}
}

func TestCmdDoctor_ReportsWhenOlderServerOmitsCounts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/hydra-info" {
			_, _ = w.Write([]byte(`{"appVersion":"0.1.0"}`))
			return
		}
		_, _ = w.Write([]byte(`{"controllers":[]}`))
	}))
	defer server.Close()

	var output bytes.Buffer
	if err := cmdDoctor(&output, []string{"--server", server.URL}); err != nil {
		t.Fatalf("older compatible server must not fail solely for omitted counts: %v", err)
	}
	if !strings.Contains(output.String(), "countCrossCheck=not-reported") {
		t.Fatalf("expected an honest omitted-count result, got: %q", output.String())
	}
}

func TestCmdDoctor_ReportsMalformedJSONAsServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	var discard bytes.Buffer
	assertCliErrorCode(t, cmdDoctor(&discard, []string{"--server", server.URL}), ExitServerError)
}

func TestCmdDoctor_ReportsUnreachableServerAsNetworkError(t *testing.T) {
	var discard bytes.Buffer
	assertCliErrorCode(t, cmdDoctor(&discard, []string{"--server", "http://127.0.0.1:1"}), ExitNetworkError)
}

func TestCmdDoctor_RejectsUnexpectedArguments(t *testing.T) {
	var discard bytes.Buffer
	assertCliErrorCode(t, cmdDoctor(&discard, []string{"unexpected"}), ExitUsageError)
}

// TestCmdDoctor_JSONOutputHealthyReportsEveryCheckPass closes 's real
// gap: a script driving `doctor` had only DOCTOR=PASS prose to parse, with no
// per-check identifier or severity. --json must report the full, real check
// list, all passing, on a healthy target.
func TestCmdDoctor_JSONOutputHealthyReportsEveryCheckPass(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/hydra-info":
			_, _ = w.Write([]byte(`{"schema_version":"1.0","appVersion":"0.2.4","remoteApiVersion":2,"controllerCount":1,"robotCount":2}`))
		case "/api/settings":
			_, _ = w.Write([]byte(`{"controllers":[{"name":"Master","robots":[{"name":"A1"},{"name":"A2"}]}]}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	var output bytes.Buffer
	if err := cmdDoctor(&output, []string{"--server", server.URL, "--json"}); err != nil {
		t.Fatalf("cmdDoctor --json returned an error against a healthy server: %v", err)
	}

	var report doctorReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("--json output is not valid JSON: %v\noutput: %s", err, output.String())
	}
	if !report.OK {
		t.Fatalf("expected ok=true against a healthy server, got report: %+v", report)
	}
	if report.AppVersion != "0.2.4" || report.Controllers != 1 || report.Robots != 2 {
		t.Fatalf("unexpected report facts: %+v", report)
	}
	if len(report.Checks) != 5 {
		t.Fatalf("expected 5 checks, got %d: %+v", len(report.Checks), report.Checks)
	}
	for _, check := range report.Checks {
		if check.Status != "pass" {
			t.Fatalf("expected every check to pass against a healthy server, got %+v", check)
		}
		if check.CheckID == "" || check.Severity == "" {
			t.Fatalf("every check must carry a checkId and severity, got %+v", check)
		}
	}
}

// TestCmdDoctor_JSONOutputCountMismatchReportsFailedCheck proves --json
// keeps the exact same failure classification as plain-text mode (the error
// and its exit code are unchanged) while additionally naming which specific
// check failed, instead of only a prose sentence.
func TestCmdDoctor_JSONOutputCountMismatchReportsFailedCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/hydra-info" {
			_, _ = w.Write([]byte(`{"appVersion":"0.2.4","controllerCount":1,"robotCount":8}`))
			return
		}
		_, _ = w.Write([]byte(`{"controllers":[{"robots":[{"name":"A1"}]}]}`))
	}))
	defer server.Close()

	var output bytes.Buffer
	err := cmdDoctor(&output, []string{"--server", server.URL, "--json"})
	assertCliErrorCode(t, err, ExitServerError)

	var report doctorReport
	if jsonErr := json.Unmarshal(output.Bytes(), &report); jsonErr != nil {
		t.Fatalf("--json output is not valid JSON even on failure: %v\noutput: %s", jsonErr, output.String())
	}
	if report.OK {
		t.Fatalf("expected ok=false on a count mismatch, got report: %+v", report)
	}
	found := false
	for _, check := range report.Checks {
		if check.CheckID == "robot-count-cross-check" {
			found = true
			if check.Status != "fail" || check.Severity != "warning" || check.Message == "" {
				t.Fatalf("unexpected robot-count-cross-check entry: %+v", check)
			}
		}
	}
	if !found {
		t.Fatalf("expected a robot-count-cross-check entry in checks: %+v", report.Checks)
	}
}

// TestCmdDoctor_JSONOutputUnreachableServerReportsCriticalFailCheck proves an
// early, network-level failure still yields valid, structured JSON (a single
// critical fail check) rather than the empty stdout plain-text mode produces
// on the same failure.
func TestCmdDoctor_JSONOutputUnreachableServerReportsCriticalFailCheck(t *testing.T) {
	var output bytes.Buffer
	err := cmdDoctor(&output, []string{"--server", "http://127.0.0.1:1", "--json"})
	assertCliErrorCode(t, err, ExitNetworkError)

	var report doctorReport
	if jsonErr := json.Unmarshal(output.Bytes(), &report); jsonErr != nil {
		t.Fatalf("--json output is not valid JSON on an unreachable server: %v\noutput: %s", jsonErr, output.String())
	}
	if report.OK {
		t.Fatalf("expected ok=false against an unreachable server, got report: %+v", report)
	}
	if len(report.Checks) != 1 {
		t.Fatalf("expected exactly one check before an unreachable server aborts the run, got %+v", report.Checks)
	}
	check := report.Checks[0]
	if check.CheckID != "hydra-info-reachable" || check.Severity != "critical" || check.Status != "fail" || check.Message == "" {
		t.Fatalf("unexpected single check: %+v", check)
	}
}
