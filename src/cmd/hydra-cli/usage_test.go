// HYDRA-UMC-TOOL-CLI - tests of the help of one command
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0 - see LICENSE
package main

import (
	"os"
	"strings"
	"testing"
)

func TestEveryCommandPrintsItsOwnUsageAndExitsOK(t *testing.T) {
	for _, name := range commandNames() {
		for _, flag := range []string{"--help", "-h"} {
			var code ExitCode
			out := captureStdout(t, func() { code = run([]string{name, flag}) })
			if code != ExitOK {
				t.Fatalf("%s %s: exit %d", name, flag, code)
			}
			if !strings.HasPrefix(out, "USAGE: hydra-cli "+name) {
				t.Fatalf("%s %s printed %q, want its own usage", name, flag, out)
			}
			if strings.Contains(out, "COMMANDS:") {
				t.Fatalf("%s %s printed the general help", name, flag)
			}
		}
	}
}

func TestHelpWinsOverTheRestOfTheArguments(t *testing.T) {
	// no robot, no server, nothing valid: the help is still printed and nothing is sent anywhere
	var code ExitCode
	out := captureStdout(t, func() { code = run([]string{"jog", "--bogus", "--help", "--server", "http://127.0.0.1:1"}) })
	if code != ExitOK || !strings.HasPrefix(out, "USAGE: hydra-cli jog ") {
		t.Fatalf("exit %d, %q", code, out)
	}
}

func TestEveryCommandOfTheDispatcherHasAUsage(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, name := range []string{"version", "status", "robots", "doctor", "claims", "claim", "release", "limits", "jog-arm", "jog", "table", "robot", "audit", "config", "shell", "completion"} {
		if !strings.Contains(text, `case "`+name+`"`) && name != "version" {
			t.Fatalf("the dispatcher has no %q", name)
		}
		if _, ok := commandUsage[name]; !ok {
			t.Fatalf("%q has no usage text", name)
		}
	}
	if len(commandUsage) != 16 {
		t.Fatalf("%d usage texts for 16 commands: %v", len(commandUsage), commandNames())
	}
}

func TestEveryOptionOfAUsageIsOneTheCommandReallyReads(t *testing.T) {
	sources := ""
	for _, file := range []string{"control.go", "record.go", "robots.go", "doctor.go", "server.go", "config.go", "shell.go", "main.go"} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		sources += string(data)
	}
	for name, text := range commandUsage {
		for _, line := range strings.Split(text, "\n") {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, "--") {
				continue
			}
			option := strings.Fields(trimmed)[0]
			if !strings.Contains(sources, `"`+option+`"`) {
				t.Errorf("%s documents %s, which no source reads", name, option)
			}
		}
	}
}

func TestAnUnknownCommandStillGetsTheGeneralHelp(t *testing.T) {
	var code ExitCode
	_ = captureStdout(t, func() { code = run([]string{"nonsense", "--help"}) })
	if code != ExitUsageError {
		t.Fatalf("exit %d, want the usage error for an unknown command", code)
	}
}
