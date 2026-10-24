// =============================================================================
// HYDRA-UMC-TOOL-CLI - Sessions: login, logout and token reuse: cmd/hydra-cli/session.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0-or-later - see LICENSE
// =============================================================================
// A login costs the server: it is rate limited (5 per 15 minutes per address) and every login
// leaves a refresh token behind that lives for 90 days. A CLI that logs in on every run, in a
// cron job or a loop, runs into the limit and fills the server with tokens nobody will use.
// This file is how it stops doing that:
//
//   - `hydra-cli login` signs in once and saves the token and the refresh token to a file only
//     the user can read (HYDRA_CLI_TOKEN_FILE, "tokenFile" of a --config file, or
//     hydra-cli/session.json in the user's config directory). Every later command uses it, and
//     refreshes it by itself with POST /api/refresh when the access token has expired.
//   - `hydra-cli logout` revokes the refresh token on the server and removes the file.
//   - With only HYDRA_CLI_USER and HYDRA_CLI_PASSWORD in the environment the CLI still logs in,
//     but at most once per process (a `shell` session shares one login) and it revokes the
//     refresh token it obtained when the process ends.
//
// The order in which a credential is looked for: HYDRA_CLI_TOKEN, then the saved session of this
// server, then the user and password of the environment.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const tokenFileEnv = "HYDRA_CLI_TOKEN_FILE"

// activeTokenFile is the "tokenFile" of the --config file of the command being run (the environment wins).
var activeTokenFile string

// savedSession is the content of the session file.
type savedSession struct {
	Server       string `json:"server"`
	Token        string `json:"token"`
	RefreshToken string `json:"refreshToken,omitempty"`
	Role         string `json:"role,omitempty"`
	SavedAt      string `json:"savedAt"`
}

// credentialSource says where a credential came from, so a refusal knows what can be done about it.
type credentialSource int

const (
	sourceNone credentialSource = iota
	sourceEnvToken
	sourceSavedSession
	sourceEnvLogin
)

type credential struct {
	token  string
	source credentialSource
}

// liveLogin is a login this process made with the user and password of the environment.
type liveLogin struct {
	server  string
	token   string
	refresh string
}

var liveLogins = struct {
	mu    sync.Mutex
	items map[string]*liveLogin
}{items: map[string]*liveLogin{}}

func loginKey(server, user string) string { return server + "\x00" + user }

// resetSessions forgets the logins of this process without revoking them (tests).
func resetSessions() {
	liveLogins.mu.Lock()
	defer liveLogins.mu.Unlock()
	liveLogins.items = map[string]*liveLogin{}
}

// sessionPath is where the session is saved.
func sessionPath() (string, error) {
	if path := strings.TrimSpace(os.Getenv(tokenFileEnv)); path != "" {
		return path, nil
	}
	if activeTokenFile != "" {
		return activeTokenFile, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("no place for the session file: %w (set %s)", err, tokenFileEnv)
	}
	return filepath.Join(dir, "hydra-cli", "session.json"), nil
}

// readSession returns the saved session when there is one for this server.
func readSession(server string) (savedSession, bool) {
	path, err := sessionPath()
	if err != nil {
		return savedSession{}, false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return savedSession{}, false
	}
	var saved savedSession
	if json.Unmarshal(raw, &saved) != nil || saved.Token == "" || saved.Server != server {
		return savedSession{}, false
	}
	return saved, true
}

// writeSession saves the session so that only the user can read it: written to a new file next to the
// target (mode 0600) and renamed over it, so a crash never leaves half a file.
func writeSession(saved savedSession) error {
	path, err := sessionPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".session-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0o600); err != nil && !isWindows() {
		temp.Close()
		return err
	}
	if _, err := temp.Write(append(encoded, '\n')); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, path)
}

func isWindows() bool { return os.PathSeparator == '\\' }

// postJSON is a POST with a JSON body and no credential, with a bounded read.
func postJSON(client *http.Client, server, path string, payload any) (int, []byte, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, newCliError(ExitGeneralError, err)
	}
	resp, err := client.Post(strings.TrimRight(server, "/")+path, "application/json", bytes.NewReader(encoded))
	if err != nil {
		return 0, nil, newCliError(ExitNetworkError, fmt.Errorf("could not reach %s: %w", server, err))
	}
	defer resp.Body.Close()
	body, err := readBody(resp.Body)
	if err != nil {
		return 0, nil, bodyReadError(server, "the response of "+path, err)
	}
	return resp.StatusCode, body, nil
}

// loginResult is the answer of POST /api/login and POST /api/refresh.
type loginResult struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refreshToken"`
	Role         string `json:"role"`
}

// signIn is POST /api/login with the error mapping every login uses.
func signIn(client *http.Client, server, user, password string) (loginResult, error) {
	if err := requireSecureCredentials(server); err != nil {
		return loginResult{}, err
	}
	status, body, err := postJSON(client, server, "/api/login", map[string]string{"username": user, "password": password})
	if err != nil {
		return loginResult{}, err
	}
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return loginResult{}, newCliError(ExitAuthError, fmt.Errorf("%s refused the login for %s: check %s and %s", server, user, userEnv, passwordEnv))
	case status == http.StatusTooManyRequests:
		return loginResult{}, newCliError(ExitServerError, fmt.Errorf("%s/api/login is rate limited (HTTP 429): too many logins; wait and retry, run `hydra-cli login` once and reuse the saved session, or set %s to a token instead of logging in on every run", server, tokenEnv))
	case status != http.StatusOK:
		return loginResult{}, newCliError(ExitServerError, fmt.Errorf("%s/api/login replied with HTTP %d", server, status))
	}
	var result loginResult
	if err := json.Unmarshal(body, &result); err != nil || result.Token == "" {
		return loginResult{}, newCliError(ExitServerError, errors.New("the login response carried no token"))
	}
	return result, nil
}

// resolveCredential finds the credential for a request: the token of the environment, the saved session of this
// server, or a login with the user and password of the environment (made once per process).
func resolveCredential(client *http.Client, server string) (credential, error) {
	if token := strings.TrimSpace(os.Getenv(tokenEnv)); token != "" {
		return credential{token: token, source: sourceEnvToken}, nil
	}
	if saved, ok := readSession(server); ok {
		return credential{token: saved.Token, source: sourceSavedSession}, nil
	}
	user, password := os.Getenv(userEnv), os.Getenv(passwordEnv)
	if user == "" || password == "" {
		return credential{}, nil
	}
	liveLogins.mu.Lock()
	cached := liveLogins.items[loginKey(server, user)]
	liveLogins.mu.Unlock()
	if cached != nil {
		return credential{token: cached.token, source: sourceEnvLogin}, nil
	}
	result, err := signIn(client, server, user, password)
	if err != nil {
		return credential{}, err
	}
	liveLogins.mu.Lock()
	liveLogins.items[loginKey(server, user)] = &liveLogin{server: server, token: result.Token, refresh: result.RefreshToken}
	liveLogins.mu.Unlock()
	return credential{token: result.Token, source: sourceEnvLogin}, nil
}

// resolveToken is resolveCredential for callers that only need the token.
func resolveToken(client *http.Client, server string) (string, error) {
	cred, err := resolveCredential(client, server)
	return cred.token, err
}

// shouldRecover says whether an answer means "the access token ran out" (worth one refresh), as opposed to a
// refusal by role, which a new token would not change.
func shouldRecover(status int, body []byte) bool {
	if status == http.StatusUnauthorized {
		return true
	}
	if status != http.StatusForbidden {
		return false
	}
	var parsed struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(body, &parsed)
	return parsed.Code == "TOKEN_EXPIRED"
}

// recoverCredential gets a new access token after the server refused the one used, when that is possible without
// a person: a saved session is refreshed with its refresh token; a login of the environment is made again. A token
// given in HYDRA_CLI_TOKEN is the operator's, and is never replaced.
func recoverCredential(client *http.Client, server string, used credential) (credential, bool) {
	switch used.source {
	case sourceSavedSession:
		saved, ok := readSession(server)
		if !ok || saved.RefreshToken == "" {
			return credential{}, false
		}
		if requireSecureCredentials(server) != nil {
			return credential{}, false
		}
		status, body, err := postJSON(client, server, "/api/refresh", map[string]string{"refreshToken": saved.RefreshToken})
		if err != nil || status != http.StatusOK {
			return credential{}, false
		}
		var result loginResult
		if json.Unmarshal(body, &result) != nil || result.Token == "" {
			return credential{}, false
		}
		saved.Token = result.Token
		if result.RefreshToken != "" {
			saved.RefreshToken = result.RefreshToken
		}
		saved.SavedAt = time.Now().UTC().Format(time.RFC3339)
		if err := writeSession(saved); err != nil {
			fmt.Fprintf(os.Stderr, "hydra-cli: warning: the refreshed session could not be saved: %v\n", err)
		}
		return credential{token: result.Token, source: sourceSavedSession}, true
	case sourceEnvLogin:
		user, password := os.Getenv(userEnv), os.Getenv(passwordEnv)
		if user == "" || password == "" {
			return credential{}, false
		}
		result, err := signIn(client, server, user, password)
		if err != nil {
			return credential{}, false
		}
		liveLogins.mu.Lock()
		if old := liveLogins.items[loginKey(server, user)]; old != nil && old.refresh != "" {
			// the refresh token of the login that ran out is revoked with the others when the process ends
			liveLogins.items[loginKey(server, user)+"\x00old"] = old
		}
		liveLogins.items[loginKey(server, user)] = &liveLogin{server: server, token: result.Token, refresh: result.RefreshToken}
		liveLogins.mu.Unlock()
		return credential{token: result.Token, source: sourceEnvLogin}, true
	}
	return credential{}, false
}

// endSessions revokes the refresh tokens of the logins this process made with the environment's user and password,
// so a run does not leave one behind. Best effort: a server that does not answer is left alone.
func endSessions() {
	liveLogins.mu.Lock()
	items := liveLogins.items
	liveLogins.items = map[string]*liveLogin{}
	liveLogins.mu.Unlock()
	if len(items) == 0 {
		return
	}
	client := newClient(3 * time.Second)
	for _, login := range items {
		if login.refresh == "" {
			continue
		}
		_, _, _ = postJSON(&client, login.server, "/api/logout", map[string]string{"refreshToken": login.refresh})
	}
}

// cmdLogin implements `hydra-cli login [--user NAME] [--password-stdin]`.
func cmdLogin(w io.Writer, in io.Reader, args []string) error {
	server, timeout, rest, err := resolveTarget(args)
	if err != nil {
		return err
	}
	user, rest, err := takeFlag(rest, "--user")
	if err != nil {
		return err
	}
	fromStdin, rest := takeSwitch(rest, "--password-stdin")
	if err := rejectExtraArgs("login", rest); err != nil {
		return err
	}
	if user == "" {
		user = os.Getenv(userEnv)
	}
	password := os.Getenv(passwordEnv)
	if fromStdin {
		line, readErr := bufio.NewReader(io.LimitReader(in, 4096)).ReadString('\n')
		if readErr != nil && line == "" {
			return newCliError(ExitUsageError, errors.New("--password-stdin: no password was read from the standard input"))
		}
		password = strings.TrimRight(line, "\r\n")
	}
	if user == "" || password == "" {
		return newCliError(ExitUsageError, fmt.Errorf("login needs a user (--user, or %s) and a password (%s, or --password-stdin)", userEnv, passwordEnv))
	}
	client := newClient(timeout)
	result, err := signIn(&client, server, user, password)
	if err != nil {
		return err
	}
	saved := savedSession{Server: server, Token: result.Token, RefreshToken: result.RefreshToken, Role: result.Role, SavedAt: time.Now().UTC().Format(time.RFC3339)}
	if err := writeSession(saved); err != nil {
		// the login worked but cannot be kept: do not leave its refresh token behind
		if result.RefreshToken != "" {
			_, _, _ = postJSON(&client, server, "/api/logout", map[string]string{"refreshToken": result.RefreshToken})
		}
		return newCliError(ExitGeneralError, fmt.Errorf("the session could not be saved: %w", err))
	}
	path, _ := sessionPath()
	role := ""
	if result.Role != "" {
		role = " as " + printable(result.Role)
	}
	fmt.Fprintf(w, "Signed in to %s%s. Session saved to %s; later commands use it. `hydra-cli logout` ends it.\n", server, role, path)
	if result.RefreshToken == "" {
		fmt.Fprintln(w, "This server gave no refresh token: when the token expires, sign in again.")
	}
	return nil
}

// cmdLogout implements `hydra-cli logout`: revoke the refresh token on the server and remove the session file.
func cmdLogout(w io.Writer, args []string) error {
	server, timeout, rest, err := resolveTarget(args)
	if err != nil {
		return err
	}
	if err := rejectExtraArgs("logout", rest); err != nil {
		return err
	}
	saved, ok := readSession(server)
	if !ok {
		fmt.Fprintf(w, "No saved session for %s.\n", server)
		return nil
	}
	revoked := "the server was not asked to revoke it (no refresh token)"
	if saved.RefreshToken != "" {
		client := newClient(timeout)
		if status, _, postErr := postJSON(&client, server, "/api/logout", map[string]string{"refreshToken": saved.RefreshToken}); postErr != nil || status != http.StatusOK {
			revoked = "the server could not be told to revoke the refresh token (it expires by itself)"
		} else {
			revoked = "the refresh token was revoked on the server"
		}
	}
	path, _ := sessionPath()
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return newCliError(ExitGeneralError, fmt.Errorf("the session file %s could not be removed: %w", path, err))
	}
	fmt.Fprintf(w, "Signed out of %s: %s; %s removed.\n", server, revoked, path)
	return nil
}
