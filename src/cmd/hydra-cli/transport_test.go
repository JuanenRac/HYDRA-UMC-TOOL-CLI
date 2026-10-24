// =============================================================================
// HYDRA-UMC-TOOL-CLI - cmd/hydra-cli/transport_test.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0-or-later - see LICENSE
// =============================================================================
package main

import (
	"bytes"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestEveryRequestIdentifiesTheCLI(t *testing.T) {
	clearCredentials(t)
	var agent, client atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agent.Store(r.Header.Get("User-Agent"))
		client.Store(r.Header.Get("X-Hydra-Client"))
		_, _ = w.Write([]byte(`{"appVersion":"0.8.0"}`))
	}))
	defer server.Close()
	if err := cmdStatusTo(&bytes.Buffer{}, []string{"--server", server.URL}); err != nil {
		t.Fatal(err)
	}
	if got := agent.Load(); got != "hydra-cli/"+Version {
		t.Fatalf("User-Agent = %v", got)
	}
	if got := client.Load(); got != "cli" {
		t.Fatalf("X-Hydra-Client = %v", got)
	}
}

// writeServerCA writes the certificate of a test TLS server as a PEM file and returns its path.
func writeServerCA(t *testing.T, server *httptest.Server) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ca.pem")
	block := &pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAnHTTPSServerWithAPrivateCAIsTrustedOnlyWithTheCAFile(t *testing.T) {
	clearCredentials(t)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"appVersion":"0.8.0","product":"HYDRA-UMC"}`))
	}))
	defer server.Close()

	// without the CA the certificate is not trusted: a network error, not a silent pass
	t.Setenv(caFileEnv, "")
	err := cmdStatusTo(&bytes.Buffer{}, []string{"--server", server.URL})
	if exitCodeFor(err) != ExitNetworkError {
		t.Fatalf("without the CA: err = %v, want a network error", err)
	}

	// with it, the same command works
	t.Setenv(caFileEnv, writeServerCA(t, server))
	var out bytes.Buffer
	if err := cmdStatusTo(&out, []string{"--server", server.URL}); err != nil || !strings.Contains(out.String(), "0.8.0") {
		t.Fatalf("with the CA: %v %q", err, out.String())
	}
}

func TestTheCAFileCanComeFromTheConfigFile(t *testing.T) {
	clearCredentials(t)
	t.Setenv(caFileEnv, "")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"appVersion":"0.8.0"}`))
	}))
	defer server.Close()
	ca := writeServerCA(t, server)
	config := writeConfigFile(t, `{"server":"`+server.URL+`","timeoutSec":5,"caFile":`+quoteJSON(ca)+`}`)
	var out bytes.Buffer
	if err := cmdStatusTo(&out, []string{"--config", config}); err != nil || !strings.Contains(out.String(), "0.8.0") {
		t.Fatalf("%v %q", err, out.String())
	}
	// and the validator reports it
	var validated bytes.Buffer
	if err := cmdConfig(&validated, []string{"validate", "--config", config}); err != nil || !strings.Contains(validated.String(), "caFile") {
		t.Fatalf("config validate: %v %q", err, validated.String())
	}
}

func quoteJSON(s string) string {
	var b bytes.Buffer
	b.WriteByte('"')
	for _, r := range s {
		if r == '\\' || r == '"' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('"')
	return b.String()
}

func TestABadCAFileIsAConfigurationErrorBeforeAnythingIsSent(t *testing.T) {
	clearCredentials(t)
	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { atomic.AddInt32(&hits, 1) }))
	defer server.Close()

	t.Setenv(caFileEnv, filepath.Join(t.TempDir(), "missing.pem"))
	err := cmdStatusTo(&bytes.Buffer{}, []string{"--server", server.URL})
	if exitCodeFor(err) != ExitConfigError {
		t.Fatalf("missing CA file: %v", err)
	}
	notPEM := filepath.Join(t.TempDir(), "not.pem")
	_ = os.WriteFile(notPEM, []byte("hello"), 0o600)
	t.Setenv(caFileEnv, notPEM)
	err = cmdStatusTo(&bytes.Buffer{}, []string{"--server", server.URL})
	if exitCodeFor(err) != ExitConfigError || !strings.Contains(err.Error(), "no PEM certificate") {
		t.Fatalf("CA file that is not PEM: %v", err)
	}
	if atomic.LoadInt32(&hits) != 0 {
		t.Fatal("a request was sent with a CA file that cannot be used")
	}
	// a config file with a bad caFile does not validate
	config := writeConfigFile(t, `{"server":"http://127.0.0.1:1","timeoutSec":5,"caFile":`+quoteJSON(notPEM)+`}`)
	if err := cmdConfig(&bytes.Buffer{}, []string{"validate", "--config", config}); exitCodeFor(err) != ExitConfigError {
		t.Fatalf("config validate with a bad caFile: %v", err)
	}
}

func TestAllowInsecureFromTheConfigFileOverridesTheEnvironment(t *testing.T) {
	clearCredentials(t)
	t.Setenv(allowInsecureEnv, "1")
	config := writeConfigFile(t, `{"server":"http://192.0.2.1:3000","timeoutSec":1,"allowInsecure":false}`)
	t.Setenv(tokenEnv, "some-token")
	err := cmdRobots(&bytes.Buffer{}, []string{"--config", config})
	if exitCodeFor(err) != ExitConfigError || !strings.Contains(err.Error(), "refusing to send") {
		t.Fatalf("allowInsecure=false in the config must win over the environment: %v", err)
	}
	if got := settingsFromEnvironment(); !got.AllowInsecure {
		t.Fatal("the environment value was not read")
	}
}

func TestTheTimeoutFlagSetsTheTimeoutAndIsValidated(t *testing.T) {
	clearCredentials(t)
	t.Setenv("HYDRA_CLI_TIMEOUT_SEC", "")
	_, timeout, _, err := resolveTarget([]string{"--timeout", "9"})
	if err != nil || timeout.Seconds() != 9 {
		t.Fatalf("timeout = %v, %v", timeout, err)
	}
	config := writeConfigFile(t, `{"server":"http://127.0.0.1:1","timeoutSec":30}`)
	if _, timeout, _, err = resolveTarget([]string{"--config", config, "--timeout", "2"}); err != nil || timeout.Seconds() != 2 {
		t.Fatalf("--timeout must win over the config file: %v %v", timeout, err)
	}
	for _, bad := range []string{"0", "-1", "abc", "3601", "1.5", ""} {
		_, _, _, err := resolveTarget([]string{"--timeout", bad})
		if bad == "" {
			continue
		}
		if exitCodeFor(err) != ExitUsageError {
			t.Errorf("--timeout %q: %v", bad, err)
		}
	}
	if _, _, _, err := resolveTarget([]string{"--timeout"}); exitCodeFor(err) != ExitUsageError {
		t.Errorf("--timeout without a value: %v", err)
	}
}

func TestStatusCanPrintJSON(t *testing.T) {
	clearCredentials(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"appVersion":"0.8.0","product":"HYDRA-UMC","hostname":"cm5","remoteApiVersion":2}`))
	}))
	defer server.Close()
	var out bytes.Buffer
	if err := cmdStatusTo(&out, []string{"--server", server.URL, "--json"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"appVersion": "0.8.0"`, `"hostname": "cm5"`, `"remoteApiVersion": 2`} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %s in %s", want, out.String())
		}
	}
}

func TestDoctorReadsACountMismatchOnceMoreBeforeFailing(t *testing.T) {
	clearCredentials(t)
	var settingsReads int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/hydra-info":
			_, _ = w.Write([]byte(`{"schema_version":"1.0","appVersion":"0.8.0","controllerCount":1,"robotCount":2}`))
		case "/api/settings":
			// the disk lags behind memory on the first read and has caught up on the second
			if atomic.AddInt32(&settingsReads, 1) == 1 {
				_, _ = w.Write([]byte(`{"controllers":[{"name":"M","robots":[{"name":"A"}]}]}`))
				return
			}
			_, _ = w.Write([]byte(`{"controllers":[{"name":"M","robots":[{"name":"A"},{"name":"B"}]}]}`))
		}
	}))
	defer server.Close()
	var out bytes.Buffer
	if err := cmdDoctor(&out, []string{"--server", server.URL}); err != nil || !strings.Contains(out.String(), "DOCTOR=PASS") {
		t.Fatalf("a transient mismatch must pass after the second read: %v %q", err, out.String())
	}
	if atomic.LoadInt32(&settingsReads) != 2 {
		t.Fatalf("settings read %d times, want 2", settingsReads)
	}
}

func TestDoctorStillFailsWhenTheMismatchStays(t *testing.T) {
	clearCredentials(t)
	var settingsReads int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/hydra-info":
			_, _ = w.Write([]byte(`{"schema_version":"1.0","appVersion":"0.8.0","controllerCount":1,"robotCount":5}`))
		case "/api/settings":
			atomic.AddInt32(&settingsReads, 1)
			_, _ = w.Write([]byte(`{"controllers":[{"name":"M","robots":[{"name":"A"}]}]}`))
		}
	}))
	defer server.Close()
	err := cmdDoctor(&bytes.Buffer{}, []string{"--server", server.URL})
	if exitCodeFor(err) != ExitServerError || !strings.Contains(err.Error(), "robot count mismatch") {
		t.Fatalf("err = %v", err)
	}
	if atomic.LoadInt32(&settingsReads) != 2 {
		t.Fatalf("settings read %d times, want exactly 2 (not a loop)", settingsReads)
	}
}

func TestCompletionHasFishAndPowerShellAndTheOptions(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		var out bytes.Buffer
		if err := cmdCompletion(&out, []string{shell}); err != nil {
			t.Fatalf("%s: %v", shell, err)
		}
		text := out.String()
		for _, command := range topLevelCommands {
			if !strings.Contains(text, command) {
				t.Errorf("%s completion misses the command %q", shell, command)
			}
		}
		for _, option := range []string{"--server", "--timeout"} {
			if shell == "fish" {
				option = option[2:] // fish writes -l server
			}
			if !strings.Contains(text, option) {
				t.Errorf("%s completion misses the option %q", shell, option)
			}
		}
		for _, sub := range append(append([]string{}, fleetSubcommands...), manifestSubcommands...) {
			if !strings.Contains(text, sub) {
				t.Errorf("%s completion misses the subcommand %q", shell, sub)
			}
		}
	}
}

func TestEveryDispatchedCommandIsInTheCompletionList(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, name := range topLevelCommands {
		listed[name] = true
	}
	for _, line := range strings.Split(string(source), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "case \"") || !strings.HasSuffix(trimmed, "\":") {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(trimmed, "case \""), "\":")
		if strings.Contains(name, "\"") { // the aliases: case "version", "-v", "--version":
			continue
		}
		if !listed[name] {
			t.Errorf("the dispatcher handles %q but the completion does not list it", name)
		}
	}
}
