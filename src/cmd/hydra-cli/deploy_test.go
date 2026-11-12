// =============================================================================
// HYDRA-UMC-TOOL-CLI - cmd/hydra-cli/deploy_test.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0-or-later - see LICENSE
// =============================================================================
// deploy against a stand-in updater: a process of the test itself (the test binary started again) that answers `deploy-plan` the way the
// real updater does. No device and no real updater is involved.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

const fakePlanJSON = `{"schema_version":"1.0","steps":1,"projects":[
 {"project":"HYDRA-UMC-SERVER","state":"deploy","checkoutVersion":"0.9.1","deviceVersion":"0.9.0","unit":"hydra-umc-server.service","target":"/opt/hydra-umc/server","command":["x"],"note":""},
 {"project":"HYDRA-UMC-DATALAKE","state":"current","checkoutVersion":"0.3.0","deviceVersion":"0.3.0","unit":null,"target":null,"command":null,"note":""}]}`

// TestHelperUpdater is not a test: it is the stand-in updater, run when the test binary is started again with HYDRA_TEST_UPDATER set.
func TestHelperUpdater(t *testing.T) {
	mode := os.Getenv("HYDRA_TEST_UPDATER")
	if mode == "" {
		t.Skip("only the stand-in updater runs this")
	}
	args := os.Args
	for i, a := range args {
		if a == "--" {
			args = args[i+1:]
			break
		}
	}
	if logPath := os.Getenv("HYDRA_TEST_UPDATER_LOG"); logPath != "" {
		f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		fmt.Fprintln(f, strings.Join(args, " "))
		_ = f.Close()
	}
	apply := false
	for _, a := range args {
		if a == "--apply" {
			apply = true
		}
	}
	switch mode {
	case "plan":
		fmt.Print(fakePlanJSON)
		if apply {
			fmt.Fprintln(os.Stderr, "HYDRA-UMC-SERVER: deployed - the unit answered its health route")
		}
	case "applyfails":
		fmt.Print(fakePlanJSON)
		if apply {
			fmt.Fprintln(os.Stderr, "HYDRA-UMC-SERVER: rolled-back - the unit did not come back\nsecond line")
			os.Exit(1)
		}
	case "garbage":
		fmt.Print("this is not a plan")
	case "oldversion":
		fmt.Print(`{"schema_version":"2.0","steps":0,"projects":[]}`)
	case "wrongcount":
		fmt.Print(`{"schema_version":"1.0","steps":5,"projects":[]}`)
	case "failing":
		fmt.Fprintln(os.Stderr, "ERROR: the table of installers cannot be used")
		os.Exit(1)
	case "noop":
		fmt.Print(`{"schema_version":"1.0","steps":0,"projects":[{"project":"A","state":"current","checkoutVersion":"1","deviceVersion":"1","unit":null,"target":null,"command":null,"note":""}]}`)
	case "hostile":
		fmt.Print(`{"schema_version":"1.0","steps":0,"projects":[{"project":"A\u001b[31mred\nFORGED LINE","state":"current","checkoutVersion":"1","deviceVersion":"1","unit":null,"target":null,"command":null,"note":""}]}`)
	case "hang":
		time.Sleep(30 * time.Second)
	case "flood":
		chunk := strings.Repeat("x", 1<<20)
		for i := 0; i < 6; i++ {
			fmt.Print(chunk)
		}
	}
	os.Exit(0)
}

// updaterCommand is the --updater value that starts this test binary as the stand-in.
func updaterCommand(t *testing.T, mode string) string {
	t.Helper()
	t.Setenv("HYDRA_TEST_UPDATER", mode)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return "'" + exe + "' -test.run=^TestHelperUpdater$ --"
}

func TestDeploy_WithoutApplyOnlyShowsThePlan(t *testing.T) {
	log := t.TempDir() + "/calls.log"
	t.Setenv("HYDRA_TEST_UPDATER_LOG", log)
	var out bytes.Buffer
	if err := cmdDeploy(&out, []string{"--updater", updaterCommand(t, "plan"), "--workspace", "/work space"}); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	text := out.String()
	for _, want := range []string{"HYDRA-UMC-SERVER", "deploy", "0.9.0", "0.9.1", "hydra-umc-server.service", "HYDRA-UMC-DATALAKE", "current", "1 step(s) to deploy. Nothing was changed"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in\n%s", want, text)
		}
	}
	calls, _ := os.ReadFile(log)
	lines := strings.Split(strings.TrimSpace(string(calls)), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], "deploy-plan --json --workspace /work space") || strings.Contains(lines[0], "--apply") {
		t.Fatalf("the updater was called with: %q", lines)
	}
}

func TestDeploy_ApplyNeedsYesAndYesNeedsApply(t *testing.T) {
	log := t.TempDir() + "/calls.log"
	t.Setenv("HYDRA_TEST_UPDATER_LOG", log)
	cmd := updaterCommand(t, "plan")
	for _, args := range [][]string{{"--apply"}, {"--yes"}} {
		if err := cmdDeploy(&bytes.Buffer{}, append([]string{"--updater", cmd}, args...)); exitCodeFor(err) != ExitUsageError {
			t.Errorf("%v: %v", args, err)
		}
	}
	if _, err := os.Stat(log); err == nil {
		t.Fatal("the updater was started although the confirmation was wrong")
	}
}

func TestDeploy_ApplyRunsThePlanThenTheSameCommandWithApplyAndYes(t *testing.T) {
	log := t.TempDir() + "/calls.log"
	t.Setenv("HYDRA_TEST_UPDATER_LOG", log)
	var out bytes.Buffer
	if err := cmdDeploy(&out, []string{"--updater", updaterCommand(t, "plan"), "--apply", "--yes", "--install-plan", "plan.json", "--device-root", "/opt/x"}); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	calls, _ := os.ReadFile(log)
	lines := strings.Split(strings.TrimSpace(string(calls)), "\n")
	if len(lines) != 2 || strings.Contains(lines[0], "--apply") || !strings.HasSuffix(lines[1], "--apply --yes") || !strings.Contains(lines[1], "--install-plan plan.json") || !strings.Contains(lines[1], "--device-root /opt/x") {
		t.Fatalf("calls: %q", lines)
	}
	if !strings.Contains(out.String(), "deployed - the unit answered its health route") || strings.Contains(out.String(), "Nothing was changed") {
		t.Fatalf("%s", out.String())
	}
}

func TestDeploy_ANonEmptyPlanIsNotAppliedWhenThereIsNothingToDo(t *testing.T) {
	log := t.TempDir() + "/calls.log"
	t.Setenv("HYDRA_TEST_UPDATER_LOG", log)
	var out bytes.Buffer
	if err := cmdDeploy(&out, []string{"--updater", updaterCommand(t, "noop"), "--apply", "--yes"}); err != nil {
		t.Fatal(err)
	}
	calls, _ := os.ReadFile(log)
	if strings.Count(string(calls), "\n") != 1 || !strings.Contains(out.String(), "nothing to deploy") {
		t.Fatalf("calls %q out %q", calls, out.String())
	}
}

func TestDeploy_AFailedDeploymentIsAnErrorWithTheReasonOfTheUpdater(t *testing.T) {
	var out bytes.Buffer
	err := cmdDeploy(&out, []string{"--updater", updaterCommand(t, "applyfails"), "--apply", "--yes"})
	if exitCodeFor(err) != ExitServerError || !strings.Contains(err.Error(), "rolled-back") {
		t.Fatalf("%v", err)
	}
	if !strings.Contains(out.String(), "rolled-back") {
		t.Fatalf("the message of the updater was not shown:\n%s", out.String())
	}
}

func TestDeploy_AnUnusablePlanOrUpdaterIsRefused(t *testing.T) {
	for mode, want := range map[string]string{"garbage": "did not print a plan", "oldversion": "version", "wrongcount": "counts its steps wrongly", "failing": "the table of installers cannot be used"} {
		err := cmdDeploy(&bytes.Buffer{}, []string{"--updater", updaterCommand(t, mode)})
		if exitCodeFor(err) != ExitServerError || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v", mode, err)
		}
	}
	if err := cmdDeploy(&bytes.Buffer{}, []string{"--updater", "/definitely/not/there/updater"}); exitCodeFor(err) != ExitGeneralError {
		t.Errorf("a missing updater: %v", err)
	}
}

func TestDeploy_TheTimeoutKillsAnUpdaterThatDoesNotAnswer(t *testing.T) {
	start := time.Now()
	err := cmdDeploy(&bytes.Buffer{}, []string{"--updater", updaterCommand(t, "hang"), "--timeout", "1"})
	if exitCodeFor(err) != ExitNetworkError || time.Since(start) > 15*time.Second {
		t.Fatalf("%v after %s", err, time.Since(start))
	}
}

func TestDeploy_TooMuchOutputIsRefused(t *testing.T) {
	err := cmdDeploy(&bytes.Buffer{}, []string{"--updater", updaterCommand(t, "flood")})
	if exitCodeFor(err) != ExitServerError {
		t.Fatalf("%v", err)
	}
}

func TestDeploy_TextFromTheUpdaterCannotForgeLinesOrDriveTheTerminal(t *testing.T) {
	var out bytes.Buffer
	if err := cmdDeploy(&out, []string{"--updater", updaterCommand(t, "hostile")}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "\x1b") || strings.Contains(out.String(), "\nFORGED LINE") {
		t.Fatalf("%q", out.String())
	}
}

func TestDeploy_JSONHasAStableShape(t *testing.T) {
	var out bytes.Buffer
	if err := cmdDeploy(&out, []string{"--updater", updaterCommand(t, "plan"), "--apply", "--yes", "--json"}); err != nil {
		t.Fatal(err)
	}
	var doc struct {
		SchemaVersion string   `json:"schema_version"`
		Command       string   `json:"command"`
		OK            bool     `json:"ok"`
		Applied       bool     `json:"applied"`
		Steps         int      `json:"steps"`
		Messages      []string `json:"messages"`
		Projects      []struct {
			Project       string  `json:"project"`
			State         string  `json:"state"`
			DeviceVersion *string `json:"device_version"`
			Unit          *string `json:"unit"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	if doc.SchemaVersion != "1.0" || doc.Command != "deploy" || !doc.OK || !doc.Applied || doc.Steps != 1 || len(doc.Projects) != 2 || doc.Projects[1].Unit != nil || *doc.Projects[0].DeviceVersion != "0.9.0" || len(doc.Messages) != 1 {
		t.Fatalf("%+v", doc)
	}
}

func TestDeploy_TheUpdaterCommandComesFromTheOptionOrTheEnvironment(t *testing.T) {
	log := t.TempDir() + "/calls.log"
	t.Setenv("HYDRA_TEST_UPDATER_LOG", log)
	t.Setenv(updaterEnv, updaterCommand(t, "plan"))
	if err := cmdDeploy(&bytes.Buffer{}, nil); err != nil {
		t.Fatalf("from the environment: %v", err)
	}
	if calls, _ := os.ReadFile(log); !strings.Contains(string(calls), "deploy-plan") {
		t.Fatal("the updater of the environment did not run")
	}
}

func TestDeploy_SplitCommandLine(t *testing.T) {
	good := map[string][]string{
		`updater`:                  {"updater"},
		`ssh user@device  updater`: {"ssh", "user@device", "updater"},
		`"C:\Program Files\x" --a`: nil, // a backslash inside double quotes is kept as it is
		`a 'b c' "d e" f\ g`:       {"a", "b c", "d e", "f g"},
		`  spaced   out  `:         {"spaced", "out"},
		`empty "" word`:            {"empty", "", "word"},
	}
	for text, want := range good {
		got, err := splitCommandLine(text)
		if err != nil {
			t.Errorf("%q: %v", text, err)
			continue
		}
		if want != nil && strings.Join(got, "|") != strings.Join(want, "|") {
			t.Errorf("%q = %q, want %q", text, got, want)
		}
	}
	for _, text := range []string{`a "b`, `a 'b`, `a\`} {
		if _, err := splitCommandLine(text); err == nil {
			t.Errorf("%q was accepted", text)
		}
	}
	if err := cmdDeploy(&bytes.Buffer{}, []string{"--updater", "   "}); exitCodeFor(err) != ExitUsageError {
		t.Errorf("an empty command: %v", err)
	}
	if err := cmdDeploy(&bytes.Buffer{}, []string{"--updater", `a "b`}); exitCodeFor(err) != ExitUsageError {
		t.Errorf("an unclosed quote: %v", err)
	}
	for _, bad := range []string{"0", "-3", "3601", "x"} {
		if err := cmdDeploy(&bytes.Buffer{}, []string{"--updater", "u", "--timeout", bad}); exitCodeFor(err) != ExitUsageError {
			t.Errorf("--timeout %s: %v", bad, err)
		}
	}
}
