// =============================================================================
// HYDRA-UMC-TOOL-CLI - cmd/hydra-cli/config_test.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0 - see LICENSE
// =============================================================================
package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfigFile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hydra-cli.json")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("writing fixture config: %v", err)
	}
	return path
}

func TestValidateConfig_Valid(t *testing.T) {
	cfg := Config{Server: "http://localhost:3000", TimeoutSec: 5}
	if err := validateConfig(cfg); err != nil {
		t.Fatalf("validateConfig(valid) returned an error: %v", err)
	}
}

func TestValidateConfig_RejectsEmptyServer(t *testing.T) {
	err := validateConfig(Config{Server: "", TimeoutSec: 5})
	if err == nil {
		t.Fatal("expected an error for an empty server, got nil")
	}
}

func TestValidateConfig_RejectsMalformedServerURL(t *testing.T) {
	for _, bad := range []string{"not-a-url", "localhost:3000", "://missing-scheme"} {
		if err := validateConfig(Config{Server: bad, TimeoutSec: 5}); err == nil {
			t.Fatalf("expected an error for malformed server %q, got nil", bad)
		}
	}
}

func TestValidateConfig_RejectsNonPositiveTimeout(t *testing.T) {
	for _, bad := range []int{0, -1, -100} {
		err := validateConfig(Config{Server: "http://localhost:3000", TimeoutSec: bad})
		if err == nil {
			t.Fatalf("expected an error for timeoutSec=%d, got nil", bad)
		}
	}
}

func TestLoadConfig_MissingFileIsConfigError(t *testing.T) {
	_, err := loadConfig(filepath.Join(t.TempDir(), "does-not-exist.json"))
	assertCliErrorCode(t, err, ExitConfigError)
}

func TestLoadConfig_MalformedJSONIsConfigError(t *testing.T) {
	path := writeConfigFile(t, "{ not valid json")
	_, err := loadConfig(path)
	assertCliErrorCode(t, err, ExitConfigError)
}

func TestLoadConfig_SchemaViolationIsConfigError(t *testing.T) {
	path := writeConfigFile(t, `{"server": "", "timeoutSec": 5}`)
	_, err := loadConfig(path)
	assertCliErrorCode(t, err, ExitConfigError)
}

func TestLoadConfig_RealValidFile(t *testing.T) {
	path := writeConfigFile(t, `{"server": "http://localhost:3000", "timeoutSec": 5}`)
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig(valid file) returned an error: %v", err)
	}
	if cfg.Server != "http://localhost:3000" || cfg.TimeoutSec != 5 {
		t.Fatalf("loadConfig returned unexpected values: %+v", cfg)
	}
}

func TestCmdConfig_NoArgsIsUsageError(t *testing.T) {
	var buf bytes.Buffer
	err := cmdConfig(&buf, nil)
	assertCliErrorCode(t, err, ExitUsageError)
}

func TestCmdConfig_MissingPathFlagIsUsageError(t *testing.T) {
	var buf bytes.Buffer
	err := cmdConfig(&buf, []string{"validate"})
	assertCliErrorCode(t, err, ExitUsageError)
}

func TestCmdConfig_UnknownSubcommandIsUsageError(t *testing.T) {
	path := writeConfigFile(t, `{"server": "http://localhost:3000", "timeoutSec": 5}`)
	var buf bytes.Buffer
	err := cmdConfig(&buf, []string{"launch-rockets", "--config", path})
	assertCliErrorCode(t, err, ExitUsageError)
}

func TestCmdConfigValidate_RealFile(t *testing.T) {
	path := writeConfigFile(t, `{"server": "http://localhost:3000", "timeoutSec": 5}`)
	var buf bytes.Buffer
	if err := cmdConfigValidate(&buf, path); err != nil {
		t.Fatalf("cmdConfigValidate(valid) returned an error: %v", err)
	}
	if !strings.Contains(buf.String(), "is valid") {
		t.Fatalf("expected a confirmation message, got: %q", buf.String())
	}
}

func TestCmdConfigApply_DryRunPrintsPreviewAndTouchesNothing(t *testing.T) {
	path := writeConfigFile(t, `{"server": "http://localhost:9999", "timeoutSec": 7}`)
	var buf bytes.Buffer
	if err := cmdConfigApply(&buf, path, true); err != nil {
		t.Fatalf("cmdConfigApply(dry-run) returned an error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "DRY RUN") || !strings.Contains(out, "http://localhost:9999") {
		t.Fatalf("dry-run output missing expected content: %q", out)
	}
}

func TestCmdConfigApply_WithoutDryRunIsNotImplemented(t *testing.T) {
	path := writeConfigFile(t, `{"server": "http://localhost:3000", "timeoutSec": 5}`)
	var buf bytes.Buffer
	err := cmdConfigApply(&buf, path, false)
	assertCliErrorCode(t, err, ExitNotImplemented)
}

func TestCmdConfigApply_InvalidConfigIsConfigErrorEvenInDryRun(t *testing.T) {
	path := writeConfigFile(t, `{"server": "", "timeoutSec": 5}`)
	var buf bytes.Buffer
	err := cmdConfigApply(&buf, path, true)
	assertCliErrorCode(t, err, ExitConfigError)
}

func assertCliErrorCode(t *testing.T, err error, want ExitCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected an error classified %d, got nil", want)
	}
	var cliErr *CliError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected a *CliError, got %T: %v", err, err)
	}
	if cliErr.Code != want {
		t.Fatalf("CliError.Code = %d, want %d (%v)", cliErr.Code, want, err)
	}
}
