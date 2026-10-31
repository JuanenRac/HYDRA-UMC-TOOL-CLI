// =============================================================================
// HYDRA-UMC-TOOL-CLI - cmd/hydra-cli/completion_generated_test.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0-or-later - see LICENSE
// =============================================================================
package main

import (
	"bytes"
	"os/exec"
	"sort"
	"strings"
	"testing"
)

func completionScript(t *testing.T, shell string) string {
	t.Helper()
	var buf bytes.Buffer
	if err := cmdCompletion(&buf, []string{shell}); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// The options of a command come from its usage text, so a new option shows up in the scripts by itself.
func TestCompletion_OptionsAreReadFromTheUsageOfEachCommand(t *testing.T) {
	cases := map[string][]string{
		"connections":   {"--kind", "--user", "--active", "--action", "--limit", "--before", "--json", "--server", "--help"},
		"plan-simulate": {"--plan", "--planner", "--json", "--config", "--timeout", "--help"},
		"jog":           {"--absolute", "--if-match", "--json", "--help"},
		"scan":          {"--ports", "--scheme", "--allow-public-range", "--json"},
		"stop-all":      {"--yes", "--json"},
	}
	for command, want := range cases {
		got := strings.Join(optionsOf(command), " ")
		for _, option := range want {
			if !strings.Contains(" "+got+" ", " "+option+" ") {
				t.Errorf("%s: %s is missing from %s", command, option, got)
			}
		}
	}
	for _, command := range topLevelCommands {
		options := optionsOf(command)
		if !sort.StringsAreSorted(options) || options[0] == "" {
			t.Errorf("%s: options are not sorted", command)
		}
	}
}

// Every command, every subcommand, every option and every value of the table is in each of the four scripts.
func TestCompletion_EveryShellCoversTheWholeTable(t *testing.T) {
	for _, shell := range completionShells {
		script := completionScript(t, shell)
		for _, command := range topLevelCommands {
			if !strings.Contains(script, command) {
				t.Errorf("%s: no %s", shell, command)
			}
			for _, sub := range subcommandsOf(command) {
				if !strings.Contains(script, sub) {
					t.Errorf("%s: no %s %s", shell, command, sub)
				}
			}
			for _, option := range optionsOf(command) {
				name := option
				if shell == "fish" {
					name = option[2:]
				}
				if !strings.Contains(script, name) {
					t.Errorf("%s: no %s of %s", shell, option, command)
				}
			}
		}
		for option, values := range optionValues {
			for _, value := range values {
				if !strings.Contains(script, value) {
					t.Errorf("%s: no value %s of %s", shell, value, option)
				}
			}
		}
	}
}

// The scripts are the same on every run (they are generated from maps).
func TestCompletion_IsDeterministic(t *testing.T) {
	for _, shell := range completionShells {
		first := completionScript(t, shell)
		for i := 0; i < 5; i++ {
			if completionScript(t, shell) != first {
				t.Fatalf("%s: the script changes between runs", shell)
			}
		}
	}
}

// No script reads a file, runs a program or calls the network when it is only loaded.
func TestCompletion_ScriptsRunNothingWhenLoaded(t *testing.T) {
	for _, shell := range completionShells {
		script := completionScript(t, shell)
		for _, banned := range []string{"curl ", "wget ", "http://", "https://", "eval ", "$(hydra-cli", "`"} {
			if strings.Contains(script, banned) {
				t.Errorf("%s: the script contains %q", shell, banned)
			}
		}
	}
}

// bashComplete loads the bash script in a real bash, as the user would, and asks it for the words after `words`.
func bashComplete(t *testing.T, script string, words ...string) []string {
	t.Helper()
	quoted := make([]string, len(words))
	for i, word := range words {
		quoted[i] = "'" + word + "'"
	}
	program := script + "\nCOMP_WORDS=(hydra-cli " + strings.Join(quoted, " ") + ")\nCOMP_CWORD=" + itoa(int64(len(words))) + "\n_hydra_cli_complete\nprintf '%s\\n' \"${COMPREPLY[@]}\"\n"
	cmd := exec.Command("bash", "-s")
	cmd.Stdin = strings.NewReader(program)
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("bash is not usable here: %v", err)
	}
	var replies []string
	for _, line := range strings.Split(strings.TrimSpace(strings.ReplaceAll(string(out), "\r", "")), "\n") {
		if line != "" {
			replies = append(replies, line)
		}
	}
	sort.Strings(replies)
	return replies
}

func TestCompletion_BashAnswersLikeTheTable(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("no bash")
	}
	script := completionScript(t, "bash")
	check := func(want []string, words ...string) {
		t.Helper()
		got := bashComplete(t, script, words...)
		sort.Strings(want)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%v: got %v, want %v", words, got, want)
		}
	}
	check([]string{"connections", "completion", "config"}, "co")
	check([]string{"liveness"}, "liv")
	check([]string{"clients", "events"}, "connections", "")
	check([]string{"events"}, "connections", "e")
	check(connectionKinds, "connections", "--kind", "")
	check([]string{"android"}, "connections", "--kind", "an")
	check(connectionActions, "connections", "events", "--action", "")
	check([]string{"--planner", "--plan"}, "plan-simulate", "--pla")
	check([]string{"on", "off"}, "jog-arm", "1", "")
	check([]string{"bash", "zsh", "fish", "powershell"}, "completion", "")
	check([]string{"validate", "apply"}, "config", "")
	check([]string{"--yes"}, "stop-all", "--y")
	check([]string{"--json"}, "claim", "3", "--j")
}
