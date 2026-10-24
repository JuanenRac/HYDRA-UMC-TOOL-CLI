// =============================================================================
// HYDRA-UMC-TOOL-CLI - cmd/hydra-cli/session_test.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0-or-later - see LICENSE
// =============================================================================
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
)

// sessionServer behaves like the auth routes of HYDRA-UMC-SERVER: login hands out an access token and a refresh token,
// refresh trades a refresh token for a new pair (rotating it), logout revokes, and /api/settings wants the current access token.
type sessionServer struct {
	*httptest.Server
	logins, refreshes, logouts int32
	currentToken               atomic.Value // string
	currentRefresh             atomic.Value // string
	revoked                    atomic.Value // string: the last refresh token revoked
	generation                 int32
}

func newSessionServer(t *testing.T) *sessionServer {
	t.Helper()
	s := &sessionServer{}
	s.currentToken.Store("")
	s.currentRefresh.Store("")
	s.revoked.Store("")
	issue := func() (string, string) {
		n := atomic.AddInt32(&s.generation, 1)
		token, refresh := "access-"+string(rune('a'+n)), "refresh-"+string(rune('a'+n))
		s.currentToken.Store(token)
		s.currentRefresh.Store(refresh)
		return token, refresh
	}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		switch r.URL.Path {
		case "/api/login":
			atomic.AddInt32(&s.logins, 1)
			if body["username"] != "operator" || body["password"] != "right-password" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"Invalid credentials"}`))
				return
			}
			token, refresh := issue()
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "token": token, "refreshToken": refresh, "role": "operator"})
		case "/api/refresh":
			atomic.AddInt32(&s.refreshes, 1)
			if body["refreshToken"] != s.currentRefresh.Load().(string) || body["refreshToken"] == "" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"Invalid or expired refresh token"}`))
				return
			}
			token, refresh := issue()
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "token": token, "refreshToken": refresh, "role": "operator"})
		case "/api/logout":
			atomic.AddInt32(&s.logouts, 1)
			s.revoked.Store(body["refreshToken"])
			_, _ = w.Write([]byte(`{"success":true}`))
		case "/api/settings":
			header := r.Header.Get("Authorization")
			if header == "" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"Access denied: No token provided"}`))
				return
			}
			if header != "Bearer "+s.currentToken.Load().(string) {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error":"Access denied: token expired, please log in again","code":"TOKEN_EXPIRED"}`))
				return
			}
			_, _ = w.Write([]byte(`{"controllers":[{"name":"Master","robots":[{"name":"R1","online":true}]}]}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(s.Close)
	return s
}

func useTempSession(t *testing.T) string {
	t.Helper()
	clearCredentials(t)
	path := filepath.Join(t.TempDir(), "session.json")
	t.Setenv(tokenFileEnv, path)
	return path
}

func TestLogin_SavesASessionOnlyTheUserCanRead(t *testing.T) {
	path := useTempSession(t)
	server := newSessionServer(t)
	t.Setenv(userEnv, "operator")
	t.Setenv(passwordEnv, "right-password")

	var out bytes.Buffer
	if err := cmdLogin(&out, strings.NewReader(""), []string{"--server", server.URL}); err != nil {
		t.Fatalf("cmdLogin: %v", err)
	}
	if !strings.Contains(out.String(), "Signed in") || !strings.Contains(out.String(), path) {
		t.Fatalf("unexpected output: %q", out.String())
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the session file was not written: %v", err)
	}
	var saved savedSession
	if err := json.Unmarshal(raw, &saved); err != nil || saved.Token != "access-b" || saved.RefreshToken != "refresh-b" || saved.Server != server.URL {
		t.Fatalf("saved = %+v (%v)", saved, err)
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(path)
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("the session file has mode %v, want 0600", info.Mode().Perm())
		}
	}
	if strings.Contains(out.String(), "right-password") || strings.Contains(out.String(), saved.Token) {
		t.Fatalf("login printed a secret: %q", out.String())
	}
}

func TestLogin_ThePasswordCanComeFromStandardInput(t *testing.T) {
	useTempSession(t)
	server := newSessionServer(t)
	t.Setenv(userEnv, "")
	t.Setenv(passwordEnv, "")
	if err := cmdLogin(&bytes.Buffer{}, strings.NewReader("right-password\n"), []string{"--server", server.URL, "--user", "operator", "--password-stdin"}); err != nil {
		t.Fatalf("cmdLogin: %v", err)
	}
	if atomic.LoadInt32(&server.logins) != 1 {
		t.Fatalf("logins = %d", server.logins)
	}
	// without a password there is nothing to try
	err := cmdLogin(&bytes.Buffer{}, strings.NewReader(""), []string{"--server", server.URL, "--user", "operator", "--password-stdin"})
	if exitCodeFor(err) != ExitUsageError {
		t.Fatalf("empty stdin: %v", err)
	}
}

func TestLogin_AWrongPasswordIsAnAuthErrorAndSavesNothing(t *testing.T) {
	path := useTempSession(t)
	server := newSessionServer(t)
	t.Setenv(userEnv, "operator")
	t.Setenv(passwordEnv, "wrong")
	err := cmdLogin(&bytes.Buffer{}, strings.NewReader(""), []string{"--server", server.URL})
	if exitCodeFor(err) != ExitAuthError {
		t.Fatalf("err = %v", err)
	}
	if _, statErr := os.Stat(path); statErr == nil {
		t.Fatal("a failed login left a session file")
	}
}

func TestSavedSession_IsUsedByLaterCommandsWithoutLoggingInAgain(t *testing.T) {
	useTempSession(t)
	server := newSessionServer(t)
	t.Setenv(userEnv, "operator")
	t.Setenv(passwordEnv, "right-password")
	if err := cmdLogin(&bytes.Buffer{}, strings.NewReader(""), []string{"--server", server.URL}); err != nil {
		t.Fatal(err)
	}
	// the environment no longer holds the password: only the saved session is left
	t.Setenv(userEnv, "")
	t.Setenv(passwordEnv, "")
	for i := 0; i < 5; i++ {
		var out bytes.Buffer
		if err := cmdRobots(&out, []string{"--server", server.URL}); err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
		if !strings.Contains(out.String(), "R1") {
			t.Fatalf("run %d: %q", i, out.String())
		}
	}
	if got := atomic.LoadInt32(&server.logins); got != 1 {
		t.Fatalf("logins = %d, want the one of `login` only", got)
	}
}

func TestSavedSession_IsRefreshedByItselfWhenTheAccessTokenRanOut(t *testing.T) {
	path := useTempSession(t)
	server := newSessionServer(t)
	t.Setenv(userEnv, "operator")
	t.Setenv(passwordEnv, "right-password")
	if err := cmdLogin(&bytes.Buffer{}, strings.NewReader(""), []string{"--server", server.URL}); err != nil {
		t.Fatal(err)
	}
	t.Setenv(userEnv, "")
	t.Setenv(passwordEnv, "")
	// the server stops accepting the access token the file holds, but accepts the refresh token
	server.currentToken.Store("a-newer-token-the-file-does-not-know")

	var out bytes.Buffer
	if err := cmdRobots(&out, []string{"--server", server.URL}); err != nil {
		t.Fatalf("cmdRobots: %v", err)
	}
	if !strings.Contains(out.String(), "R1") || atomic.LoadInt32(&server.refreshes) != 1 || atomic.LoadInt32(&server.logins) != 1 {
		t.Fatalf("refreshes = %d, logins = %d, output %q", server.refreshes, server.logins, out.String())
	}
	raw, _ := os.ReadFile(path)
	var saved savedSession
	_ = json.Unmarshal(raw, &saved)
	if saved.Token != server.currentToken.Load().(string) || saved.RefreshToken != server.currentRefresh.Load().(string) {
		t.Fatalf("the file was not updated with the rotated tokens: %+v", saved)
	}
}

func TestSavedSession_ARefreshTokenThatIsGoneEndsInAnAuthError(t *testing.T) {
	useTempSession(t)
	server := newSessionServer(t)
	t.Setenv(userEnv, "operator")
	t.Setenv(passwordEnv, "right-password")
	if err := cmdLogin(&bytes.Buffer{}, strings.NewReader(""), []string{"--server", server.URL}); err != nil {
		t.Fatal(err)
	}
	t.Setenv(userEnv, "")
	t.Setenv(passwordEnv, "")
	server.currentToken.Store("other")
	server.currentRefresh.Store("revoked-on-the-server")
	err := cmdRobots(&bytes.Buffer{}, []string{"--server", server.URL})
	if exitCodeFor(err) != ExitAuthError {
		t.Fatalf("err = %v, want an auth error", err)
	}
}

func TestSavedSession_OfAnotherServerIsNotSent(t *testing.T) {
	useTempSession(t)
	first := newSessionServer(t)
	second := newSessionServer(t)
	t.Setenv(userEnv, "operator")
	t.Setenv(passwordEnv, "right-password")
	if err := cmdLogin(&bytes.Buffer{}, strings.NewReader(""), []string{"--server", first.URL}); err != nil {
		t.Fatal(err)
	}
	t.Setenv(userEnv, "")
	t.Setenv(passwordEnv, "")
	err := cmdRobots(&bytes.Buffer{}, []string{"--server", second.URL})
	if exitCodeFor(err) != ExitAuthError || !strings.Contains(err.Error(), "requires a login") {
		t.Fatalf("the session of another server was used: %v", err)
	}
}

func TestTokenInTheEnvironmentWinsOverTheSavedSession(t *testing.T) {
	useTempSession(t)
	server := newSessionServer(t)
	t.Setenv(userEnv, "operator")
	t.Setenv(passwordEnv, "right-password")
	if err := cmdLogin(&bytes.Buffer{}, strings.NewReader(""), []string{"--server", server.URL}); err != nil {
		t.Fatal(err)
	}
	t.Setenv(userEnv, "")
	t.Setenv(passwordEnv, "")
	t.Setenv(tokenEnv, "a-token-the-server-refuses")
	err := cmdRobots(&bytes.Buffer{}, []string{"--server", server.URL})
	if exitCodeFor(err) != ExitAuthError {
		t.Fatalf("err = %v", err)
	}
	if atomic.LoadInt32(&server.refreshes) != 0 {
		t.Fatal("a token the operator gave was replaced by a refresh")
	}
}

func TestLogout_RevokesTheRefreshTokenAndRemovesTheFile(t *testing.T) {
	path := useTempSession(t)
	server := newSessionServer(t)
	t.Setenv(userEnv, "operator")
	t.Setenv(passwordEnv, "right-password")
	if err := cmdLogin(&bytes.Buffer{}, strings.NewReader(""), []string{"--server", server.URL}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := cmdLogout(&out, []string{"--server", server.URL}); err != nil {
		t.Fatalf("cmdLogout: %v", err)
	}
	if server.revoked.Load().(string) != "refresh-b" || !strings.Contains(out.String(), "revoked") {
		t.Fatalf("revoked = %v, output %q", server.revoked.Load(), out.String())
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatal("the session file is still there")
	}
	out.Reset()
	if err := cmdLogout(&out, []string{"--server", server.URL}); err != nil || !strings.Contains(out.String(), "No saved session") {
		t.Fatalf("second logout: %v %q", err, out.String())
	}
}

func TestLoginWithUserAndPasswordOfTheEnvironment_HappensOncePerProcessAndIsEndedAtExit(t *testing.T) {
	useTempSession(t)
	server := newSessionServer(t)
	t.Setenv(userEnv, "operator")
	t.Setenv(passwordEnv, "right-password")
	for i := 0; i < 4; i++ {
		if err := cmdRobots(&bytes.Buffer{}, []string{"--server", server.URL}); err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
	}
	if got := atomic.LoadInt32(&server.logins); got != 1 {
		t.Fatalf("logins = %d, want one for the whole process", got)
	}
	if atomic.LoadInt32(&server.logouts) != 0 {
		t.Fatal("the login was ended before the process did")
	}
	endSessions()
	if atomic.LoadInt32(&server.logouts) != 1 || server.revoked.Load().(string) != "refresh-b" {
		t.Fatalf("logouts = %d, revoked = %v", server.logouts, server.revoked.Load())
	}
	endSessions() // nothing left: no second call
	if atomic.LoadInt32(&server.logouts) != 1 {
		t.Fatal("a login was ended twice")
	}
}

func TestLoginOfTheEnvironment_IsRepeatedOnceWhenTheTokenRanOut(t *testing.T) {
	useTempSession(t)
	server := newSessionServer(t)
	t.Setenv(userEnv, "operator")
	t.Setenv(passwordEnv, "right-password")
	if err := cmdRobots(&bytes.Buffer{}, []string{"--server", server.URL}); err != nil {
		t.Fatal(err)
	}
	server.currentToken.Store("expired-for-the-first-one") // the server no longer takes the token this process holds
	var out bytes.Buffer
	if err := cmdRobots(&out, []string{"--server", server.URL}); err != nil || !strings.Contains(out.String(), "R1") {
		t.Fatalf("the second run should log in again by itself: %v %q", err, out.String())
	}
	if got := atomic.LoadInt32(&server.logins); got != 2 {
		t.Fatalf("logins = %d, want 2 (the first and one repeat)", got)
	}
	// both refresh tokens, the one that ran out and the new one, are revoked when the process ends
	endSessions()
	if got := atomic.LoadInt32(&server.logouts); got != 2 {
		t.Fatalf("logouts = %d, want 2", got)
	}
}

func TestAnExpiredSessionFileWithoutARefreshTokenStaysAnAuthError(t *testing.T) {
	path := useTempSession(t)
	server := newSessionServer(t)
	data, _ := json.Marshal(savedSession{Server: server.URL, Token: "stale"})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	err := cmdRobots(&bytes.Buffer{}, []string{"--server", server.URL})
	if exitCodeFor(err) != ExitAuthError || atomic.LoadInt32(&server.refreshes) != 0 {
		t.Fatalf("err = %v, refreshes = %d", err, server.refreshes)
	}
}

func TestShouldRecover_OnlyForARunOutTokenNotForAForbiddenRole(t *testing.T) {
	cases := []struct {
		status int
		body   string
		want   bool
	}{
		{401, `{}`, true},
		{403, `{"code":"TOKEN_EXPIRED"}`, true},
		{403, `{"code":"VIEWER_READ_ONLY"}`, false},
		{403, `{"error":"admin privileges required"}`, false},
		{403, `not json`, false},
		{200, `{}`, false},
		{500, `{}`, false},
	}
	for _, c := range cases {
		if got := shouldRecover(c.status, []byte(c.body)); got != c.want {
			t.Errorf("shouldRecover(%d, %s) = %v, want %v", c.status, c.body, got, c.want)
		}
	}
}

func TestACredentialIsNeverSentOverPlainHTTPToAnotherMachine(t *testing.T) {
	useTempSession(t)
	activeTransport = transportSettings{}
	t.Setenv(allowInsecureEnv, "")
	t.Setenv(tokenEnv, "some-token")
	// 192.0.2.1 is a documentation address: nothing may even be dialled
	err := cmdRobots(&bytes.Buffer{}, []string{"--server", "http://192.0.2.1:3000", "--timeout", "1"})
	if exitCodeFor(err) != ExitConfigError || !strings.Contains(err.Error(), "refusing to send") {
		t.Fatalf("err = %v, want the refusal (and no network attempt)", err)
	}
	// the same for a login with a user and a password
	t.Setenv(tokenEnv, "")
	t.Setenv(userEnv, "operator")
	t.Setenv(passwordEnv, "right-password")
	err = cmdRobots(&bytes.Buffer{}, []string{"--server", "http://192.0.2.1:3000", "--timeout", "1"})
	if exitCodeFor(err) != ExitConfigError || !strings.Contains(err.Error(), "refusing to send") {
		t.Fatalf("login: err = %v", err)
	}
}
