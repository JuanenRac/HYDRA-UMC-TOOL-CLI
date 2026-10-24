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
//	hydra-cli login                      a session saved to a file only the user can read (see session.go)
//
// The token wins when several are set, then the saved session. A credential is never sent over plain
// http to another machine unless the operator allowed it (see transport.go).
package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
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

// getAuthenticated is GET path against server with the configured credential.
// A 401/403 maps to ExitAuthError with a message that says what to set. When the
// server says the access token ran out and the CLI can get a new one without a
// person (a saved session with a refresh token, or the user and password of the
// environment), it does so once and repeats the request.
func getAuthenticated(client *http.Client, server, path string) (int, []byte, error) {
	cred, err := resolveCredential(client, server)
	if err != nil {
		return 0, nil, err
	}
	status, body, err := getWithToken(client, server, path, cred.token)
	if err != nil {
		return status, body, err
	}
	if cred.token != "" && shouldRecover(status, body) {
		if next, ok := recoverCredential(client, server, cred); ok {
			cred = next
			status, body, err = getWithToken(client, server, path, cred.token)
			if err != nil {
				return status, body, err
			}
		}
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		if cred.token == "" {
			return status, body, newCliError(ExitAuthError, fmt.Errorf("%s%s requires a login: set %s, or %s and %s, or run `hydra-cli login`", server, path, tokenEnv, userEnv, passwordEnv))
		}
		return status, body, newCliError(ExitAuthError, fmt.Errorf("%s%s refused the credential (HTTP %d): the token may be expired or invalid", server, path, status))
	}
	return status, body, nil
}

// getWithToken is one GET with the bearer token (none when empty), with a bounded read.
func getWithToken(client *http.Client, server, path, token string) (int, []byte, error) {
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(server, "/")+path, nil)
	if err != nil {
		return 0, nil, newCliError(ExitConfigError, fmt.Errorf("invalid server %q: %w", server, err))
	}
	if token != "" {
		if err := requireSecureCredentials(server); err != nil {
			return 0, nil, err
		}
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
	return resp.StatusCode, body, nil
}
