// =============================================================================
// HYDRA-UMC-TOOL-CLI - cmd/hydra-cli/server_test.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0 - see LICENSE
// =============================================================================
package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestResolveTarget_DefaultsWhenNothingSet(t *testing.T) {
	os.Unsetenv("HYDRA_CLI_SERVER")
	os.Unsetenv("HYDRA_CLI_TIMEOUT_SEC")
	server, timeout, rest, err := resolveTarget(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if server != defaultServerURL {
		t.Fatalf("expected the default server URL, got %q", server)
	}
	if timeout != defaultRequestTimeout {
		t.Fatalf("expected the default request timeout, got %v", timeout)
	}
	if len(rest) != 0 {
		t.Fatalf("expected no remaining args, got %v", rest)
	}
}

func TestResolveTarget_EnvVarOverridesDefault(t *testing.T) {
	t.Setenv("HYDRA_CLI_SERVER", "http://env-server:9000")
	server, _, _, err := resolveTarget(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if server != "http://env-server:9000" {
		t.Fatalf("expected the env var server URL, got %q", server)
	}
}

func TestResolveTarget_TimeoutEnvVarOverridesDefault(t *testing.T) {
	os.Unsetenv("HYDRA_CLI_SERVER")
	t.Setenv("HYDRA_CLI_TIMEOUT_SEC", "12")
	_, timeout, _, err := resolveTarget(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if timeout != 12*time.Second {
		t.Fatalf("expected the env var timeout, got %v", timeout)
	}
}

func TestResolveTarget_MalformedTimeoutEnvVarIsIgnored(t *testing.T) {
	os.Unsetenv("HYDRA_CLI_SERVER")
	for _, malformed := range []string{"not-a-number", "0", "-5"} {
		t.Run(malformed, func(t *testing.T) {
			t.Setenv("HYDRA_CLI_TIMEOUT_SEC", malformed)
			_, timeout, _, err := resolveTarget(nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if timeout != defaultRequestTimeout {
				t.Fatalf("expected a malformed/non-positive timeout env var to fall back to the default, got %v", timeout)
			}
		})
	}
}

func TestResolveTarget_FlagOverridesEnvVar(t *testing.T) {
	t.Setenv("HYDRA_CLI_SERVER", "http://env-server:9000")
	server, _, rest, err := resolveTarget([]string{"--server", "http://flag-server:9001", "--other-flag"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if server != "http://flag-server:9001" {
		t.Fatalf("expected the --server flag to win over the env var, got %q", server)
	}
	if !reflect.DeepEqual(rest, []string{"--other-flag"}) {
		t.Fatalf("expected --server/its value stripped from remaining args, got %v", rest)
	}
}

func TestResolveTarget_TrailingFlagsWithoutAValueAreUsageErrors(t *testing.T) {
	os.Unsetenv("HYDRA_CLI_SERVER")
	for _, flag := range []string{"--server", "--config"} {
		_, _, _, err := resolveTarget([]string{flag})
		if got := exitCodeFor(err); got != ExitUsageError {
			t.Errorf("%s with no value: exit code = %d, err = %v, want ExitUsageError", flag, got, err)
		}
	}
}

func TestResolveTarget_ServerIsNormalizedAndValidated(t *testing.T) {
	os.Unsetenv("HYDRA_CLI_SERVER")
	server, _, _, err := resolveTarget([]string{"--server", "http://host:3000/"})
	if err != nil || server != "http://host:3000" {
		t.Fatalf("trailing slash should be dropped, got %q err=%v", server, err)
	}
	for _, bad := range []string{"127.0.0.1:18999", "ftp://host", "http://", ""} {
		os.Setenv("HYDRA_CLI_SERVER", bad)
		_, _, _, err := resolveTarget(nil)
		if bad != "" && exitCodeFor(err) != ExitUsageError {
			t.Errorf("server %q: exit code = %d, want ExitUsageError (err=%v)", bad, exitCodeFor(err), err)
		}
	}
	os.Unsetenv("HYDRA_CLI_SERVER")
}

func TestStatusAndRobots_RejectUnknownArguments(t *testing.T) {
	os.Unsetenv("HYDRA_CLI_SERVER")
	if err := cmdStatus([]string{"--sever", "http://x"}); exitCodeFor(err) != ExitUsageError {
		t.Errorf("status with a typo flag: err = %v, want a usage error", err)
	}
	if err := cmdRobots(&bytes.Buffer{}, []string{"--bogus"}); exitCodeFor(err) != ExitUsageError {
		t.Errorf("robots with an unknown flag: err = %v, want a usage error", err)
	}
}

func TestStatus_RejectsAJSONThatIsNotAHydraServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	if err := cmdStatus([]string{"--server", srv.URL}); exitCodeFor(err) != ExitServerError {
		t.Errorf("status against a foreign JSON: err = %v, want ExitServerError", err)
	}
}

func TestLogin_RateLimitHasAClearMessage(t *testing.T) {
	clearCredentials(t)
	t.Setenv("HYDRA_CLI_USER", "u")
	t.Setenv("HYDRA_CLI_PASSWORD", "p")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	err := cmdRobots(&bytes.Buffer{}, []string{"--server", srv.URL})
	if err == nil || !strings.Contains(err.Error(), "rate limited") || exitCodeFor(err) != ExitServerError {
		t.Errorf("429 on login: err = %v, want a rate-limit message with ExitServerError", err)
	}
}

func TestResolveTarget_ConfigFileSetsServerAndTimeout(t *testing.T) {
	os.Unsetenv("HYDRA_CLI_SERVER")
	os.Unsetenv("HYDRA_CLI_TIMEOUT_SEC")
	path := writeConfigFile(t, `{"server": "http://fleet-config-server:7000", "timeoutSec": 15}`)

	server, timeout, rest, err := resolveTarget([]string{"--config", path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if server != "http://fleet-config-server:7000" {
		t.Fatalf("expected the config file's own server, got %q", server)
	}
	if timeout != 15*time.Second {
		t.Fatalf("expected the config file's own timeoutSec, got %v", timeout)
	}
	if len(rest) != 0 {
		t.Fatalf("expected --config/its value stripped from remaining args, got %v", rest)
	}
}

func TestResolveTarget_ExplicitServerFlagWinsOverConfigFile(t *testing.T) {
	os.Unsetenv("HYDRA_CLI_SERVER")
	path := writeConfigFile(t, `{"server": "http://fleet-config-server:7000", "timeoutSec": 15}`)

	server, timeout, _, err := resolveTarget([]string{"--config", path, "--server", "http://override:9002"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if server != "http://override:9002" {
		t.Fatalf("expected the explicit --server flag to win over the config file, got %q", server)
	}
	// --server only overrides the server, not the timeout the config file
	// already validated - there is no standalone --timeout flag to compete
	// with it.
	if timeout != 15*time.Second {
		t.Fatalf("expected the config file's own timeoutSec to still apply, got %v", timeout)
	}
}

func TestResolveTarget_ConfigFileWinsOverEnvVar(t *testing.T) {
	t.Setenv("HYDRA_CLI_SERVER", "http://env-server:9000")
	t.Setenv("HYDRA_CLI_TIMEOUT_SEC", "1")
	path := writeConfigFile(t, `{"server": "http://fleet-config-server:7000", "timeoutSec": 15}`)

	server, timeout, _, err := resolveTarget([]string{"--config", path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if server != "http://fleet-config-server:7000" {
		t.Fatalf("expected the --config file to win over HYDRA_CLI_SERVER, got %q", server)
	}
	if timeout != 15*time.Second {
		t.Fatalf("expected the --config file to win over HYDRA_CLI_TIMEOUT_SEC, got %v", timeout)
	}
}

func TestResolveTarget_MissingConfigFileIsARealConfigError(t *testing.T) {
	_, _, _, err := resolveTarget([]string{"--config", filepath.Join(t.TempDir(), "does-not-exist.json")})
	assertCliErrorCode(t, err, ExitConfigError)
}

func TestResolveTarget_InvalidConfigFileIsARealConfigError(t *testing.T) {
	path := writeConfigFile(t, `{"server": "", "timeoutSec": 5}`)
	_, _, _, err := resolveTarget([]string{"--config", path})
	assertCliErrorCode(t, err, ExitConfigError)
}
