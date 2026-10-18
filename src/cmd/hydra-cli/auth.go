// =============================================================================
// HYDRA-UMC-TOOL-CLI - Authenticated reads of HYDRA-UMC-SERVER: cmd/hydra-cli/auth.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0 - see LICENSE
// =============================================================================
// HYDRA-UMC-SERVER answers GET /api/settings only to a logged-in account (and
// redacts the secrets for anyone below administrator), so `robots` and
// `doctor` have to present a credential. GET /api/hydra-info stays public and
// `status` keeps using it without one.
//
// The credential comes from the environment, never from a flag, so it does not
// end up in a process listing or the shell history:
//
//	HYDRA_CLI_TOKEN                      a bearer token already issued by the server
//	HYDRA_CLI_USER + HYDRA_CLI_PASSWORD  the CLI logs in (POST /api/login) for that run
//
// The token wins when both are set.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const (
	tokenEnv    = "HYDRA_CLI_TOKEN"
	userEnv     = "HYDRA_CLI_USER"
	passwordEnv = "HYDRA_CLI_PASSWORD"

	// maxResponseBytes caps what the CLI reads from a server: the settings tree
	// of a large fleet is well under this, and a hostile or broken endpoint
	// must not be able to fill the operator's memory.
	maxResponseBytes = 4 << 20
)

// readBody reads at most maxResponseBytes, and refuses a larger response
// instead of silently truncating it into a JSON error.
func readBody(r io.Reader) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, maxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxResponseBytes {
		return nil, errResponseTooLarge
	}
	return body, nil
}

// errResponseTooLarge marks a response the server did send but the CLI refuses
// to hold in memory; callers classify it as a server error, not a network one.
var errResponseTooLarge = fmt.Errorf("the response is larger than %d bytes", maxResponseBytes)

// bodyReadError classifies a failed body read: an oversized response means the
// server answered (exit 5), anything else is a transport problem (exit 4).
func bodyReadError(server, what string, err error) error {
	code := ExitNetworkError
	if errors.Is(err, errResponseTooLarge) {
		code = ExitServerError
	}
	return newCliError(code, fmt.Errorf("reading %s from %s: %w", what, server, err))
}

// warnIfInsecure writes a warning to stderr when a credential is about to
// travel over plain http to a host that is not this machine.
func warnIfInsecure(server string) {
	parsed, err := url.Parse(server)
	if err != nil || parsed.Scheme != "http" {
		return
	}
	switch parsed.Hostname() {
	case "localhost", "127.0.0.1", "::1":
		return
	}
	fmt.Fprintf(os.Stderr, "hydra-cli: warning: sending credentials over plain http to %s; use https when the network is not trusted\n", parsed.Host)
}

// resolveToken returns the bearer token for an authenticated read, logging in
// when the environment holds a user and password instead of a token. It
// returns "" (and no error) when no credential is configured, so a server that
// does not require one keeps working.
func resolveToken(client *http.Client, server string) (string, error) {
	if token := strings.TrimSpace(os.Getenv(tokenEnv)); token != "" {
		return token, nil
	}
	user, password := os.Getenv(userEnv), os.Getenv(passwordEnv)
	if user == "" || password == "" {
		return "", nil
	}
	warnIfInsecure(server)
	payload, err := json.Marshal(map[string]string{"username": user, "password": password})
	if err != nil {
		return "", newCliError(ExitGeneralError, err)
	}
	resp, err := client.Post(strings.TrimRight(server, "/")+"/api/login", "application/json", bytes.NewReader(payload))
	if err != nil {
		return "", newCliError(ExitNetworkError, fmt.Errorf("could not reach %s: %w", server, err))
	}
	defer resp.Body.Close()
	body, err := readBody(resp.Body)
	if err != nil {
		return "", bodyReadError(server, "the login response", err)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return "", newCliError(ExitAuthError, fmt.Errorf("%s refused the login for %s: check %s and %s", server, user, userEnv, passwordEnv))
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return "", newCliError(ExitServerError, fmt.Errorf("%s/api/login is rate limited (HTTP 429): too many logins; wait and retry, or set %s to a token instead of logging in on every run", server, tokenEnv))
	}
	if resp.StatusCode != http.StatusOK {
		return "", newCliError(ExitServerError, fmt.Errorf("%s/api/login replied with HTTP %d", server, resp.StatusCode))
	}
	var login struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &login); err != nil || login.Token == "" {
		return "", newCliError(ExitServerError, errors.New("the login response carried no token"))
	}
	return login.Token, nil
}

// getAuthenticated is GET path against server with the configured credential.
// A 401/403 maps to ExitAuthError with a message that says what to set.
func getAuthenticated(client *http.Client, server, path string) (int, []byte, error) {
	token, err := resolveToken(client, server)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(server, "/")+path, nil)
	if err != nil {
		return 0, nil, newCliError(ExitConfigError, fmt.Errorf("invalid server %q: %w", server, err))
	}
	if token != "" {
		warnIfInsecure(server)
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, newCliError(ExitNetworkError, fmt.Errorf("could not reach %s: %w", server, err))
	}
	defer resp.Body.Close()
	body, err := readBody(resp.Body)
	if err != nil {
		return 0, nil, bodyReadError(server, "response", err)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		if token == "" {
			return resp.StatusCode, body, newCliError(ExitAuthError, fmt.Errorf("%s%s requires a login: set %s, or %s and %s", server, path, tokenEnv, userEnv, passwordEnv))
		}
		return resp.StatusCode, body, newCliError(ExitAuthError, fmt.Errorf("%s%s refused the credential (HTTP %d): the token may be expired or invalid", server, path, resp.StatusCode))
	}
	return resp.StatusCode, body, nil
}
