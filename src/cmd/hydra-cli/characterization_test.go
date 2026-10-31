// =============================================================================
// HYDRA-UMC-TOOL-CLI - cmd/hydra-cli/characterization_test.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0-or-later - see LICENSE
// =============================================================================
// Characterization tests: they fix what the command line does today (the table of commands, the help of each
// command and the text each command prints against a simulated server) in files under testdata/golden, so a
// change that adds options or commands cannot alter the text output of the existing ones without a test failing.
// A deliberate change to a golden file is made by running the tests with UPDATE_GOLDEN=1 and reading the diff.
package main

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

const goldenDir = "testdata/golden"

// checkGolden compares text with the golden file of name; UPDATE_GOLDEN=1 rewrites the file instead.
func checkGolden(t *testing.T, name, text string) {
	t.Helper()
	path := filepath.Join(goldenDir, name+".txt")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(goldenDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no golden file %s (run the tests once with UPDATE_GOLDEN=1): %v", path, err)
	}
	if strings.ReplaceAll(string(want), "\r\n", "\n") != text {
		t.Fatalf("the output of %s changed.\n--- golden ---\n%s\n--- now ---\n%s", name, want, text)
	}
}

// runCaptured runs the command line and returns its exit code, stdout and stderr, with the volatile parts
// (the server address, the version) replaced by fixed words.
func runCaptured(t *testing.T, args []string, replace map[string]string) string {
	t.Helper()
	var code ExitCode
	var stderr string
	stdout := captureStdout(t, func() {
		stderr = captureStderr(t, func() { code = run(args) })
	})
	text := "exit=" + itoa(int64(code)) + "\n--stdout--\n" + stdout + "--stderr--\n" + stderr
	for from, to := range replace {
		text = strings.ReplaceAll(text, from, to)
	}
	return strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), Version, "VERSION")
}

// TestCharacterization_CommandTable fixes the table of commands: every case of the dispatcher is in the table
// the completions use, and every command of the table has a help text of its own (but help itself).
func TestCharacterization_CommandTable(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]bool{}
	for _, match := range regexp.MustCompile(`(?m)^\tcase ((?:"[^"]+"(?:, )?)+):`).FindAllStringSubmatch(string(source), -1) {
		for _, name := range regexp.MustCompile(`"([^"]+)"`).FindAllStringSubmatch(match[1], -1) {
			if !strings.HasPrefix(name[1], "-") {
				cases[name[1]] = true
			}
		}
	}
	table := map[string]bool{}
	for _, name := range topLevelCommands {
		if table[name] {
			t.Errorf("%q is twice in the table of commands", name)
		}
		table[name] = true
		if !cases[name] {
			t.Errorf("%q is in the table of commands but the dispatcher has no case for it", name)
		}
		if _, ok := commandUsage[name]; !ok && name != "help" {
			t.Errorf("%q has no help text of its own", name)
		}
	}
	for name := range cases {
		if !table[name] {
			t.Errorf("the dispatcher handles %q but the table of commands (and the completions) lack it", name)
		}
	}
	for name := range commandUsage {
		if !table[name] {
			t.Errorf("there is a help text for %q, which is not in the table of commands", name)
		}
	}
}

// TestCharacterization_Help fixes the general help and the help of every command.
func TestCharacterization_Help(t *testing.T) {
	checkGolden(t, "help_general", runCaptured(t, []string{"help"}, nil))
	checkGolden(t, "help_no_arguments", runCaptured(t, nil, nil))
	names := commandNames()
	sort.Strings(names)
	for _, name := range names {
		checkGolden(t, "usage_"+name, runCaptured(t, []string{name, "--help"}, map[string]string{defaultServerURL: "DEFAULT_SERVER"}))
	}
	checkGolden(t, "unknown_command", runCaptured(t, []string{"frobnicate"}, nil))
}

// TestCharacterization_Completions fixes the completion scripts of the four shells.
func TestCharacterization_Completions(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		checkGolden(t, "completion_"+shell, runCaptured(t, []string{"completion", shell}, nil))
	}
	checkGolden(t, "completion_usage", runCaptured(t, []string{"completion"}, nil))
}

// TestCharacterization_TextOutput fixes the text each existing command prints against a simulated server.
func TestCharacterization_TextOutput(t *testing.T) {
	clearCredentials(t)
	t.Setenv(tokenEnv, "characterization-token")
	t.Setenv(controllerEnv, "cli-test")
	metricsPage := "# HELP hydra_uptime_seconds Uptime\n# TYPE hydra_uptime_seconds gauge\nhydra_uptime_seconds 123.5\nhydra_ws_clients 2\n"
	srv, _ := fakeServer(t, func(r *http.Request, rec recorded) (int, string) {
		switch {
		case r.URL.Path == "/api/hydra-info":
			return 200, `{"appVersion":"1.2.3","status":"ok","product":"HYDRA-UMC-SERVER","hostname":"bench","remoteApiVersion":2}`
		case r.URL.Path == "/api/settings":
			return 200, `{"controllers":[{"name":"Controller A","robots":[{"name":"Robot A1","online":true,"model":"Parol6 (6-DOF)","role":"Pnp"},{"name":"Robot A2","online":false,"model":"Parol6 (6-DOF)","role":"Idle"}]}]}`
		case r.URL.Path == "/api/claims":
			return 200, `{"claims":[]}`
		case r.URL.Path == "/api/safety-limits":
			return 200, limitsBody
		case r.URL.Path == "/api/stop-all":
			return 200, `{"success":true,"stoppedCount":1,"failedCount":0,"persisted":true,"robots":[{"robotId":1,"name":"A1","stopped":true,"valvesOff":2,"pumpsOff":1}]}`
		case r.URL.Path == "/metrics":
			return 200, metricsPage
		case r.Method == "GET" && r.URL.Path == "/api/robot/3":
			return 200, `{"robot":{"id":3,"name":"Arm","model":"Parol6 (6-DOF)","role":"Idle"},"revision":"abc123","cellId":"cell-1"}`
		case strings.HasSuffix(r.URL.Path, "/jog-arm"):
			return 200, `{"success":true,"armed":false}`
		case rec.Body["command"] == "table":
			return 200, `{"success":true,"revision":"r3","applied":{"x":10,"y":20}}`
		default:
			return 200, `{"success":true,"revision":"r2"}`
		}
	})
	dir := t.TempDir()
	configPath := filepath.Join(dir, "cli.json")
	if err := os.WriteFile(configPath, []byte(`{"server":"http://localhost:3000","timeoutSec":7}`), 0o600); err != nil {
		t.Fatal(err)
	}
	replace := map[string]string{srv.URL: "SERVER", configPath: "CONFIG_FILE"}
	commands := map[string][]string{
		"status":         {"status", "--server", srv.URL},
		"status_json":    {"status", "--server", srv.URL, "--json"},
		"robots":         {"robots", "--server", srv.URL},
		"robots_json":    {"robots", "--server", srv.URL, "--json"},
		"claims_empty":   {"claims", "--server", srv.URL},
		"robot":          {"robot", "3", "--server", srv.URL},
		"limits":         {"limits", "--robot", "1", "--server", srv.URL},
		"stop":           {"stop", "1", "--server", srv.URL},
		"stop_all":       {"stop-all", "--yes", "--server", srv.URL},
		"stop_all_no":    {"stop-all", "--server", srv.URL},
		"claim":          {"claim", "3", "--server", srv.URL},
		"release":        {"release", "3", "--server", srv.URL},
		"jog_arm_off":    {"jog-arm", "1", "off", "--server", srv.URL},
		"jog":            {"jog", "1", "x", "5", "--server", srv.URL},
		"table":          {"table", "1", "10", "20", "--server", srv.URL},
		"metrics":        {"metrics", "--server", srv.URL},
		"version":        {"version"},
		"version_flag":   {"--version"},
		"config_valid":   {"config", "validate", "--config", configPath},
		"config_dry_run": {"config", "apply", "--config", configPath, "--dry-run"},
		"config_apply":   {"config", "apply", "--config", configPath},
		"config_usage":   {"config"},
	}
	names := make([]string, 0, len(commands))
	for name := range commands {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		checkGolden(t, "output_"+name, runCaptured(t, commands[name], replace))
	}
}
