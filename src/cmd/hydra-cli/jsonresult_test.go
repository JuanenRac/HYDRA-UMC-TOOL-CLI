// =============================================================================
// HYDRA-UMC-TOOL-CLI - cmd/hydra-cli/jsonresult_test.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0-or-later - see LICENSE
// =============================================================================
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// jsonObject runs a command line through run() and returns the object it printed; stdout must be exactly one JSON value.
func jsonObject(t *testing.T, args ...string) map[string]any {
	t.Helper()
	var code ExitCode
	out := captureStdout(t, func() { code = run(args) })
	if code != ExitOK {
		t.Fatalf("%v exited %d, output:\n%s", args, code, out)
	}
	var object map[string]any
	if err := json.Unmarshal([]byte(out), &object); err != nil {
		t.Fatalf("%v did not print one JSON object: %v\n%s", args, err, out)
	}
	return object
}

func requireResult(t *testing.T, object map[string]any, command string) {
	t.Helper()
	if object["schema_version"] != jsonSchemaVersion || object["command"] != command || object["ok"] != true {
		t.Fatalf("the envelope of %q is wrong: %v", command, object)
	}
}

func jsonTestServer(t *testing.T) string {
	t.Helper()
	srv, _ := fakeServer(t, func(r *http.Request, rec recorded) (int, string) {
		switch {
		case r.URL.Path == "/api/safety-limits":
			return 200, limitsBody
		case strings.HasSuffix(r.URL.Path, "/jog-arm"):
			if rec.Body["armed"] == true {
				return 200, `{"success":true,"armed":true,"expiresAt":4102444800000}`
			}
			return 200, `{"success":true,"armed":false}`
		case rec.Body["command"] == "table":
			return 200, `{"success":true,"revision":"r3","applied":{"x":10,"y":20,"rz":5}}`
		case rec.Body["command"] == "jog" && rec.Body["params"] != nil && rec.Body["params"].(map[string]any)["amount"] == float64(7):
			return 200, `{"success":false,"warnings":[{"field":"x","code":"out-of-range","message":"too far"}]}`
		case strings.HasSuffix(r.URL.Path, "/command") || strings.HasSuffix(r.URL.Path, "/claim") || strings.HasSuffix(r.URL.Path, "/release"):
			return 200, `{"success":true,"revision":"r2"}`
		}
		return 404, `{"error":"no"}`
	})
	return srv.URL
}

func TestJSON_ActionCommandsPrintOneObject(t *testing.T) {
	clearCredentials(t)
	t.Setenv(tokenEnv, "json-test-token")
	t.Setenv(controllerEnv, "cli-test")
	server := jsonTestServer(t)

	stop := jsonObject(t, "stop", "1", "--json", "--server", server)
	requireResult(t, stop, "stop")
	if stop["robot"] != float64(1) || stop["revision"] != "r2" {
		t.Fatalf("stop: %v", stop)
	}

	claim := jsonObject(t, "claim", "3", "--json", "--server", server)
	requireResult(t, claim, "claim")
	if claim["robot"] != float64(3) || claim["controller_id"] != "cli-test" || claim["revision"] != "r2" {
		t.Fatalf("claim: %v", claim)
	}

	release := jsonObject(t, "release", "3", "--server", server, "--json")
	requireResult(t, release, "release")
	if release["robot"] != float64(3) {
		t.Fatalf("release: %v", release)
	}

	armOn := jsonObject(t, "jog-arm", "1", "on", "--json", "--server", server)
	requireResult(t, armOn, "jog-arm")
	if armOn["armed"] != true || armOn["expires_at_ms"] != float64(4102444800000) {
		t.Fatalf("jog-arm on: %v", armOn)
	}
	armOff := jsonObject(t, "jog-arm", "1", "off", "--json", "--server", server)
	if armOff["armed"] != false || armOff["expires_at_ms"] != nil {
		t.Fatalf("jog-arm off: %v", armOff)
	}

	jog := jsonObject(t, "jog", "1", "x", "5", "--absolute", "--json", "--server", server)
	requireResult(t, jog, "jog")
	if jog["axis"] != "x" || jog["amount"] != float64(5) || jog["absolute"] != true || jog["revision"] != "r2" {
		t.Fatalf("jog: %v", jog)
	}

	table := jsonObject(t, "table", "1", "10", "20", "--rz", "5", "--json", "--server", server)
	requireResult(t, table, "table")
	if table["x"] != float64(10) || table["y"] != float64(20) || table["rz"] != float64(5) || table["revision"] != "r3" {
		t.Fatalf("table: %v", table)
	}
}

// The jog the server did not apply is printed as ok=false with its warnings, and the exit code is still 8.
func TestJSON_JogWarningsAreOkFalseAndStillExit8(t *testing.T) {
	clearCredentials(t)
	t.Setenv(tokenEnv, "json-test-token")
	server := jsonTestServer(t)
	var code ExitCode
	out := captureStdout(t, func() { code = run([]string{"jog", "1", "x", "7", "--json", "--server", server}) })
	if code != ExitRefused {
		t.Fatalf("exit code = %d, want %d", code, ExitRefused)
	}
	var object map[string]any
	if err := json.Unmarshal([]byte(out), &object); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	warnings, _ := object["warnings"].([]any)
	if object["ok"] != false || len(warnings) != 1 {
		t.Fatalf("result: %v", object)
	}
}

// A refusal is never JSON: it goes to the standard error as text with its exit code, as it does without --json.
func TestJSON_FailuresStayTextOnStderr(t *testing.T) {
	clearCredentials(t)
	t.Setenv(tokenEnv, "json-test-token")
	srv, _ := fakeServer(t, func(r *http.Request, rec recorded) (int, string) {
		return 409, `{"error":"reserved","code":"other-account"}`
	})
	var code ExitCode
	var stderr string
	out := captureStdout(t, func() {
		stderr = captureStderr(t, func() { code = run([]string{"claim", "3", "--json", "--server", srv.URL}) })
	})
	if code != ExitRefused || out != "" || !strings.Contains(stderr, "reserved by another account") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out, stderr)
	}
}

func TestJSON_VersionAndConfig(t *testing.T) {
	version := jsonObject(t, "version", "--json")
	requireResult(t, version, "version")
	if version["name"] != projectName || version["version"] != Version {
		t.Fatalf("version: %v", version)
	}
	if flag := jsonObject(t, "--version", "--json"); flag["version"] != Version {
		t.Fatalf("--version --json: %v", flag)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "cli.json")
	if err := os.WriteFile(path, []byte(`{"server":"http://localhost:3000","timeoutSec":7,"allowInsecure":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	valid := jsonObject(t, "config", "validate", "--config", path, "--json")
	requireResult(t, valid, "config validate")
	if valid["server"] != "http://localhost:3000" || valid["timeout_sec"] != float64(7) || valid["ca_file"] != nil || valid["allow_insecure"] != true || valid["token_file"] != nil {
		t.Fatalf("config validate: %v", valid)
	}
	apply := jsonObject(t, "config", "apply", "--config", path, "--dry-run", "--json")
	requireResult(t, apply, "config apply")
	if apply["dry_run"] != true || apply["contacted"] != false {
		t.Fatalf("config apply: %v", apply)
	}
	// without --dry-run the apply is still refused with exit 6 and prints nothing
	var code ExitCode
	out := captureStdout(t, func() {
		_ = captureStderr(t, func() { code = run([]string{"config", "apply", "--config", path, "--json"}) })
	})
	if code != ExitNotImplemented || out != "" {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

// The login result never carries the token, the refresh token or the password.
func TestJSON_LoginAndLogoutNeverPrintCredentials(t *testing.T) {
	clearCredentials(t)
	resetSessions()
	t.Setenv(userEnv, "ana")
	t.Setenv(passwordEnv, "s3cret-password")
	srv, _ := fakeServer(t, func(r *http.Request, rec recorded) (int, string) {
		if r.URL.Path == "/api/login" {
			return 200, `{"token":"access-token-value","refreshToken":"refresh-token-value","role":"operator"}`
		}
		return 200, `{"success":true}`
	})
	login := jsonObject(t, "login", "--json", "--server", srv.URL)
	requireResult(t, login, "login")
	encoded, _ := json.Marshal(login)
	for _, secret := range []string{"access-token-value", "refresh-token-value", "s3cret-password"} {
		if bytes.Contains(encoded, []byte(secret)) {
			t.Fatalf("the login result carries %q: %s", secret, encoded)
		}
	}
	if login["role"] != "operator" || login["refresh_token_saved"] != true {
		t.Fatalf("login: %v", login)
	}
	logout := jsonObject(t, "logout", "--json", "--server", srv.URL)
	requireResult(t, logout, "logout")
	if logout["session_found"] != true || logout["revoked_on_server"] != true {
		t.Fatalf("logout: %v", logout)
	}
	again := jsonObject(t, "logout", "--json", "--server", srv.URL)
	if again["session_found"] != false || again["session_file"] != nil {
		t.Fatalf("second logout: %v", again)
	}
}

// Every command that reads or acts answers --json with valid JSON; the ones that cannot (help, shell, completion) are named.
func TestJSON_EveryDataCommandHasAJSONMode(t *testing.T) {
	exempt := map[string]bool{"help": true, "shell": true, "completion": true}
	for _, name := range topLevelCommands {
		if exempt[name] {
			continue
		}
		usage := commandUsage[name]
		if !strings.Contains(usage, "--json") {
			t.Errorf("the help of %q does not mention --json", name)
		}
	}
}
