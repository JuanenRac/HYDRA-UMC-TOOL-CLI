// =============================================================================
// HYDRA-UMC-TOOL-CLI - cmd/hydra-cli/auth_test.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0 - see LICENSE
// =============================================================================
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

const goodToken = "issued-by-the-server"

// newGuardedServer behaves like HYDRA-UMC-SERVER: /api/hydra-info is public,
// /api/settings needs a valid bearer token, and /api/login hands one out for
// the right username and password.
func newGuardedServer(t *testing.T, logins *int32, sawAuthOnInfo *int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/hydra-info":
			if r.Header.Get("Authorization") != "" && sawAuthOnInfo != nil {
				atomic.AddInt32(sawAuthOnInfo, 1)
			}
			_, _ = w.Write([]byte(`{"schema_version":"1.0","appVersion":"0.8.0","controllerCount":1,"robotCount":1}`))
		case "/api/settings":
			if r.Header.Get("Authorization") == "" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"Access denied: No token provided"}`))
				return
			}
			if r.Header.Get("Authorization") != "Bearer "+goodToken {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error":"Access denied: invalid token"}`))
				return
			}
			_, _ = w.Write([]byte(`{"controllers":[{"name":"Master","robots":[{"name":"Robot-Guarded","online":true}]}]}`))
		case "/api/login":
			if logins != nil {
				atomic.AddInt32(logins, 1)
			}
			var body struct{ Username, Password string }
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.Username != "operator" || body.Password != "right-password" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"Invalid credentials"}`))
				return
			}
			_, _ = w.Write([]byte(`{"success":true,"token":"` + goodToken + `","role":"operator"}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
}

func clearCredentials(t *testing.T) {
	t.Helper()
	resetSessions()
	t.Setenv(tokenFileEnv, filepath.Join(t.TempDir(), "no-session.json"))
	t.Setenv(tokenEnv, "")
	t.Setenv(userEnv, "")
	t.Setenv(passwordEnv, "")
}

func TestRobots_AGuardedServerWithNoCredentialIsAnAuthError(t *testing.T) {
	clearCredentials(t)
	server := newGuardedServer(t, nil, nil)
	defer server.Close()

	err := cmdRobots(&bytes.Buffer{}, []string{"--server", server.URL})
	if got := exitCodeFor(err); got != ExitAuthError {
		t.Fatalf("exit code = %d, want ExitAuthError (err = %v)", got, err)
	}
	if !strings.Contains(err.Error(), tokenEnv) || !strings.Contains(err.Error(), userEnv) {
		t.Fatalf("the error must say which variables to set, got: %v", err)
	}
}

func TestRobots_ATokenFromTheEnvironmentIsSent(t *testing.T) {
	clearCredentials(t)
	t.Setenv(tokenEnv, goodToken)
	server := newGuardedServer(t, nil, nil)
	defer server.Close()

	var out bytes.Buffer
	if err := cmdRobots(&out, []string{"--server", server.URL}); err != nil {
		t.Fatalf("cmdRobots: %v", err)
	}
	if !strings.Contains(out.String(), "Robot-Guarded") {
		t.Fatalf("expected the guarded roster, got:\n%s", out.String())
	}
}

func TestRobots_AWrongTokenIsAnAuthError(t *testing.T) {
	clearCredentials(t)
	t.Setenv(tokenEnv, "not-the-token")
	server := newGuardedServer(t, nil, nil)
	defer server.Close()

	err := cmdRobots(&bytes.Buffer{}, []string{"--server", server.URL})
	if got := exitCodeFor(err); got != ExitAuthError {
		t.Fatalf("exit code = %d, want ExitAuthError (err = %v)", got, err)
	}
	if !strings.Contains(err.Error(), "refused the credential") {
		t.Fatalf("expected the refused-credential message, got: %v", err)
	}
}

func TestRobots_LoggingInWithUserAndPassword(t *testing.T) {
	clearCredentials(t)
	t.Setenv(userEnv, "operator")
	t.Setenv(passwordEnv, "right-password")
	var logins int32
	server := newGuardedServer(t, &logins, nil)
	defer server.Close()

	var out bytes.Buffer
	if err := cmdRobots(&out, []string{"--server", server.URL}); err != nil {
		t.Fatalf("cmdRobots: %v", err)
	}
	if !strings.Contains(out.String(), "Robot-Guarded") || atomic.LoadInt32(&logins) != 1 {
		t.Fatalf("expected one login and the roster, logins = %d, output:\n%s", logins, out.String())
	}
}

func TestRobots_AWrongPasswordIsAnAuthError(t *testing.T) {
	clearCredentials(t)
	t.Setenv(userEnv, "operator")
	t.Setenv(passwordEnv, "wrong")
	server := newGuardedServer(t, nil, nil)
	defer server.Close()

	err := cmdRobots(&bytes.Buffer{}, []string{"--server", server.URL})
	if got := exitCodeFor(err); got != ExitAuthError {
		t.Fatalf("exit code = %d, want ExitAuthError (err = %v)", got, err)
	}
	if !strings.Contains(err.Error(), "refused the login") {
		t.Fatalf("expected the refused-login message, got: %v", err)
	}
}

func TestRobots_AUserWithoutAPasswordDoesNotTryToLogIn(t *testing.T) {
	clearCredentials(t)
	t.Setenv(userEnv, "operator")
	var logins int32
	server := newGuardedServer(t, &logins, nil)
	defer server.Close()

	err := cmdRobots(&bytes.Buffer{}, []string{"--server", server.URL})
	if got := exitCodeFor(err); got != ExitAuthError || atomic.LoadInt32(&logins) != 0 {
		t.Fatalf("exit code = %d, logins = %d, want ExitAuthError and no login attempt", got, logins)
	}
}

func TestRobots_TheTokenWinsOverUserAndPassword(t *testing.T) {
	clearCredentials(t)
	t.Setenv(tokenEnv, goodToken)
	t.Setenv(userEnv, "operator")
	t.Setenv(passwordEnv, "right-password")
	var logins int32
	server := newGuardedServer(t, &logins, nil)
	defer server.Close()

	if err := cmdRobots(&bytes.Buffer{}, []string{"--server", server.URL}); err != nil {
		t.Fatalf("cmdRobots: %v", err)
	}
	if atomic.LoadInt32(&logins) != 0 {
		t.Fatalf("a configured token must not trigger a login, logins = %d", logins)
	}
}

func TestDoctor_UsesTheCredentialForSettingsOnly(t *testing.T) {
	clearCredentials(t)
	t.Setenv(tokenEnv, goodToken)
	var sawAuthOnInfo int32
	server := newGuardedServer(t, nil, &sawAuthOnInfo)
	defer server.Close()

	var out bytes.Buffer
	if err := cmdDoctor(&out, []string{"--server", server.URL}); err != nil {
		t.Fatalf("cmdDoctor: %v", err)
	}
	if !strings.Contains(out.String(), "DOCTOR=PASS") {
		t.Fatalf("expected DOCTOR=PASS, got %q", out.String())
	}
	if atomic.LoadInt32(&sawAuthOnInfo) != 0 {
		t.Fatalf("the public /api/hydra-info must not receive the credential")
	}
}

func TestDoctor_AGuardedServerWithNoCredentialIsAnAuthError(t *testing.T) {
	clearCredentials(t)
	server := newGuardedServer(t, nil, nil)
	defer server.Close()

	err := cmdDoctor(&bytes.Buffer{}, []string{"--server", server.URL})
	if got := exitCodeFor(err); got != ExitAuthError {
		t.Fatalf("exit code = %d, want ExitAuthError (err = %v)", got, err)
	}
}

func TestStatus_NeverSendsTheCredential(t *testing.T) {
	clearCredentials(t)
	t.Setenv(tokenEnv, goodToken)
	var sawAuthOnInfo int32
	server := newGuardedServer(t, nil, &sawAuthOnInfo)
	defer server.Close()

	if err := cmdStatus([]string{"--server", server.URL}); err != nil {
		t.Fatalf("cmdStatus: %v", err)
	}
	if atomic.LoadInt32(&sawAuthOnInfo) != 0 {
		t.Fatalf("status must not send the credential to the public endpoint")
	}
}

func TestAnOversizedResponseIsRefused(t *testing.T) {
	clearCredentials(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		chunk := strings.Repeat("x", 1<<20)
		for i := 0; i < 6; i++ { // 6 MiB, over the 4 MiB cap
			_, _ = w.Write([]byte(chunk))
		}
	}))
	defer server.Close()

	for name, run := range map[string]func() error{
		"robots": func() error { return cmdRobots(&bytes.Buffer{}, []string{"--server", server.URL}) },
		"status": func() error { return cmdStatus([]string{"--server", server.URL}) },
	} {
		err := run()
		if got := exitCodeFor(err); got != ExitServerError || !strings.Contains(err.Error(), "larger than") {
			t.Errorf("%s: exit code = %d, err = %v, want ExitServerError naming the size cap", name, got, err)
		}
	}
}
