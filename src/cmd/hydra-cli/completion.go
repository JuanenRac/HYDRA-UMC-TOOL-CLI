// =============================================================================
// HYDRA-UMC-TOOL-CLI - Shell completion generator: cmd/hydra-cli/completion.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0 - see LICENSE
// =============================================================================
// `hydra-cli completion <bash|zsh|fish|powershell>` prints a completion script to stdout - `eval "$(hydra-cli
// completion bash)"` (or sourced from a completions directory) wires it up. Nothing here is a second list that can
// fall behind: the commands come from topLevelCommands (a test checks it against the dispatcher of main.go), the
// options of each command are read from its own usage text (usage.go, which another test checks against the
// sources), the subcommands from the small slices below and the values of the options with a closed list from
// the lists the commands themselves validate against. A command added to the table, or an option added to its
// usage, shows up in the four scripts without touching this file. No dependency, no network, no file read.
package main

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
)

// topLevelCommands mirrors run()'s own case labels in main.go, minus the flag aliases (-v/--version, -h/--help) a
// real user tab-completing a bare word would not expect to see suggested.
var topLevelCommands = []string{"version", "help", "status", "robots", "claims", "claim", "release", "limits", "jog-arm", "jog", "table", "robot", "audit", "doctor", "config", "shell", "completion", "login", "logout", "scan", "fleet", "stop", "stop-all", "metrics", "manifest", "connections", "liveness", "ecosystem-health", "plan-simulate"}

// configSubcommands mirrors config.go's own real switch.
var configSubcommands = []string{"validate", "apply"}

// fleetSubcommands, manifestSubcommands and connectionsSubcommands mirror the switches of fleet.go, manifest.go and observe.go.
var fleetSubcommands = []string{"doctor"}
var manifestSubcommands = []string{"validate"}
var connectionsSubcommands = []string{"clients", "events"}

// completionShells are the shells `completion` can print a script for.
var completionShells = []string{"bash", "zsh", "fish", "powershell"}

// commonOptions are read by resolveTarget or are shared by many commands.
var commonOptions = []string{"--server", "--config", "--timeout", "--json", "--help"}

// optionValues are the closed lists of values some options take.
var optionValues = map[string][]string{
	"--kind":   connectionKinds,
	"--action": connectionActions,
	"--scheme": {"http", "https"},
}

// subcommandsOf returns the words a command takes as its first argument (none for most).
func subcommandsOf(command string) []string {
	switch command {
	case "config":
		return configSubcommands
	case "fleet":
		return fleetSubcommands
	case "manifest":
		return manifestSubcommands
	case "connections":
		return connectionsSubcommands
	case "completion":
		return completionShells
	}
	return nil
}

// positionalValues are the closed lists some positional arguments take: the position counts from the command (1 is
// the first word after it).
var positionalValues = map[string]map[int][]string{
	"jog-arm": {2: {"on", "off"}},
}

var optionWord = regexp.MustCompile(`--[a-z][a-z0-9-]*`)

// optionsOf reads the options of a command from its usage text: the USAGE lines and the lines of the OPTIONS list
// that start with an option. The result is sorted, without duplicates, and always holds --help.
func optionsOf(command string) []string {
	found := map[string]bool{"--help": true}
	for _, line := range strings.Split(commandUsage[command], "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "USAGE:") || strings.HasPrefix(trimmed, "hydra-cli ") || strings.HasPrefix(trimmed, "--") {
			for _, option := range optionWord.FindAllString(trimmed, -1) {
				found[option] = true
			}
		}
	}
	options := make([]string, 0, len(found))
	for option := range found {
		options = append(options, option)
	}
	sort.Strings(options)
	return options
}

func cmdCompletion(w io.Writer, args []string) error {
	if len(args) != 1 {
		return newCliError(ExitUsageError, fmt.Errorf("usage: hydra-cli completion <bash|zsh|fish|powershell>"))
	}
	switch args[0] {
	case "bash":
		writeBashCompletion(w)
		return nil
	case "zsh":
		writeZshCompletion(w)
		return nil
	case "fish":
		writeFishCompletion(w)
		return nil
	case "powershell":
		writePowerShellCompletion(w)
		return nil
	default:
		return newCliError(ExitUsageError, fmt.Errorf("unsupported shell %q - expected \"bash\", \"zsh\", \"fish\" or \"powershell\"", args[0]))
	}
}

func joinWords(words []string) string {
	return strings.Join(words, " ")
}

// sortedKeys returns the keys of a map of lists in order, so the scripts are the same on every run.
func sortedKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func writeBashCompletion(w io.Writer) {
	var b strings.Builder
	b.WriteString("_hydra_cli_complete() {\n")
	b.WriteString("    local cur prev cmd opts\n")
	b.WriteString("    cur=\"${COMP_WORDS[COMP_CWORD]}\"\n")
	b.WriteString("    prev=\"${COMP_WORDS[COMP_CWORD-1]}\"\n")
	b.WriteString("    if [ \"$COMP_CWORD\" -eq 1 ]; then\n")
	fmt.Fprintf(&b, "        COMPREPLY=($(compgen -W \"%s\" -- \"$cur\"))\n", joinWords(topLevelCommands))
	b.WriteString("        return 0\n    fi\n")
	b.WriteString("    cmd=\"${COMP_WORDS[1]}\"\n")
	b.WriteString("    case \"$prev\" in\n")
	for _, option := range sortedKeys(optionValues) {
		fmt.Fprintf(&b, "        %s) COMPREPLY=($(compgen -W \"%s\" -- \"$cur\")); return 0 ;;\n", option, joinWords(optionValues[option]))
	}
	b.WriteString("    esac\n")
	b.WriteString("    case \"$cmd\" in\n")
	for _, command := range topLevelCommands {
		subs := subcommandsOf(command)
		positional := positionalValues[command]
		if len(subs) == 0 && len(positional) == 0 {
			fmt.Fprintf(&b, "        %s) opts=\"%s\" ;;\n", command, joinWords(optionsOf(command)))
			continue
		}
		fmt.Fprintf(&b, "        %s)\n", command)
		if len(subs) > 0 {
			fmt.Fprintf(&b, "            if [ \"$COMP_CWORD\" -eq 2 ]; then COMPREPLY=($(compgen -W \"%s\" -- \"$cur\")); return 0; fi\n", joinWords(subs))
		}
		for position := 1; position <= 8; position++ {
			if values, ok := positional[position]; ok {
				fmt.Fprintf(&b, "            if [ \"$COMP_CWORD\" -eq %d ]; then COMPREPLY=($(compgen -W \"%s\" -- \"$cur\")); return 0; fi\n", position+1, joinWords(values))
			}
		}
		fmt.Fprintf(&b, "            opts=\"%s\" ;;\n", joinWords(optionsOf(command)))
	}
	fmt.Fprintf(&b, "        *) opts=\"%s\" ;;\n", joinWords(commonOptions))
	b.WriteString("    esac\n")
	b.WriteString("    case \"$cur\" in\n        -*) COMPREPLY=($(compgen -W \"$opts\" -- \"$cur\")) ;;\n    esac\n}\n")
	b.WriteString("complete -F _hydra_cli_complete hydra-cli\n")
	io.WriteString(w, b.String())
}

func writeZshCompletion(w io.Writer) {
	var b strings.Builder
	b.WriteString("#compdef hydra-cli\n_hydra_cli() {\n")
	fmt.Fprintf(&b, "    local -a top_level\n    top_level=(%s)\n", joinWords(topLevelCommands))
	b.WriteString("    if (( CURRENT == 2 )); then\n        compadd -a top_level\n        return\n    fi\n")
	b.WriteString("    case ${words[CURRENT-1]} in\n")
	for _, option := range sortedKeys(optionValues) {
		fmt.Fprintf(&b, "        %s) compadd -- %s; return ;;\n", option, joinWords(optionValues[option]))
	}
	b.WriteString("    esac\n")
	b.WriteString("    case ${words[2]} in\n")
	for _, command := range topLevelCommands {
		subs := subcommandsOf(command)
		positional := positionalValues[command]
		fmt.Fprintf(&b, "        %s)\n", command)
		if len(subs) > 0 {
			fmt.Fprintf(&b, "            if (( CURRENT == 3 )); then compadd -- %s; return; fi\n", joinWords(subs))
		}
		for position := 1; position <= 8; position++ {
			if values, ok := positional[position]; ok {
				fmt.Fprintf(&b, "            if (( CURRENT == %d )); then compadd -- %s; return; fi\n", position+2, joinWords(values))
			}
		}
		fmt.Fprintf(&b, "            if [[ ${words[CURRENT]} == -* ]]; then compadd -- %s; fi ;;\n", joinWords(optionsOf(command)))
	}
	fmt.Fprintf(&b, "        *) if [[ ${words[CURRENT]} == -* ]]; then compadd -- %s; fi ;;\n", joinWords(commonOptions))
	b.WriteString("    esac\n}\ncompdef _hydra_cli hydra-cli\n")
	io.WriteString(w, b.String())
}

func writeFishCompletion(w io.Writer) {
	var b strings.Builder
	b.WriteString("# fish completion for hydra-cli: hydra-cli completion fish | source\n")
	b.WriteString("complete -c hydra-cli -f\n")
	fmt.Fprintf(&b, "complete -c hydra-cli -n \"__fish_use_subcommand\" -a \"%s\"\n", joinWords(topLevelCommands))
	for _, command := range topLevelCommands {
		condition := fmt.Sprintf("__fish_seen_subcommand_from %s", command)
		if subs := subcommandsOf(command); len(subs) > 0 {
			fmt.Fprintf(&b, "complete -c hydra-cli -n \"%s\" -a \"%s\"\n", condition, joinWords(subs))
		}
		if command == "jog-arm" {
			fmt.Fprintf(&b, "complete -c hydra-cli -n \"%s\" -a \"on off\"\n", condition)
		}
		for _, option := range optionsOf(command) {
			if values, ok := optionValues[option]; ok {
				fmt.Fprintf(&b, "complete -c hydra-cli -n \"%s\" -l %s -x -a \"%s\"\n", condition, option[2:], joinWords(values))
			} else {
				fmt.Fprintf(&b, "complete -c hydra-cli -n \"%s\" -l %s\n", condition, option[2:])
			}
		}
	}
	io.WriteString(w, b.String())
}

func writePowerShellCompletion(w io.Writer) {
	var b strings.Builder
	b.WriteString("# PowerShell completion for hydra-cli: hydra-cli completion powershell | Out-String | Invoke-Expression\n")
	b.WriteString("Register-ArgumentCompleter -Native -CommandName hydra-cli -ScriptBlock {\n")
	b.WriteString("    param($wordToComplete, $commandAst, $cursorPosition)\n")
	b.WriteString("    $words = @($commandAst.CommandElements | ForEach-Object { $_.ToString() })\n")
	fmt.Fprintf(&b, "    $top = '%s' -split ' '\n", joinWords(topLevelCommands))
	b.WriteString("    $subs = @{\n")
	for _, command := range topLevelCommands {
		if subs := subcommandsOf(command); len(subs) > 0 {
			fmt.Fprintf(&b, "        '%s' = '%s' -split ' '\n", command, joinWords(subs))
		}
	}
	b.WriteString("    }\n    $options = @{\n")
	for _, command := range topLevelCommands {
		fmt.Fprintf(&b, "        '%s' = '%s' -split ' '\n", command, joinWords(optionsOf(command)))
	}
	b.WriteString("    }\n    $values = @{\n")
	for _, option := range sortedKeys(optionValues) {
		fmt.Fprintf(&b, "        '%s' = '%s' -split ' '\n", option, joinWords(optionValues[option]))
	}
	b.WriteString("    }\n")
	b.WriteString("    $isNewWord = -not $wordToComplete\n")
	b.WriteString("    $count = $words.Count + $(if ($isNewWord) { 1 } else { 0 })\n")
	b.WriteString("    $previous = if ($isNewWord) { $words[-1] } elseif ($words.Count -ge 2) { $words[-2] } else { '' }\n")
	b.WriteString("    $candidates = @()\n")
	b.WriteString("    if ($count -le 2) {\n        $candidates = $top\n")
	b.WriteString("    } elseif ($values.ContainsKey($previous)) {\n        $candidates = $values[$previous]\n")
	b.WriteString("    } elseif ($count -eq 3 -and $subs.ContainsKey($words[1])) {\n        $candidates = $subs[$words[1]]\n")
	b.WriteString("    } elseif ($count -eq 4 -and $words[1] -eq 'jog-arm') {\n        $candidates = @('on', 'off')\n")
	b.WriteString("    } elseif ($options.ContainsKey($words[1])) {\n        $candidates = $options[$words[1]]\n")
	fmt.Fprintf(&b, "    } else {\n        $candidates = '%s' -split ' '\n    }\n", joinWords(commonOptions))
	b.WriteString("    $candidates | Where-Object { $_ -like \"$wordToComplete*\" } | ForEach-Object {\n")
	b.WriteString("        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)\n    }\n}\n")
	io.WriteString(w, b.String())
}
