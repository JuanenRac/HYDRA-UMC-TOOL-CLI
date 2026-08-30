// =============================================================================
// HYDRA-UMC-TOOL-CLI - Read-only diagnostic tests: cmd/hydra-cli/doctor_test.go
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
