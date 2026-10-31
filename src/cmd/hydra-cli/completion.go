// =============================================================================
// HYDRA-UMC-TOOL-CLI - Shell completion generator: cmd/hydra-cli/completion.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0 - see LICENSE
// =============================================================================
// `hydra-cli completion <bash|zsh>` prints a real, static completion
// script to stdout - `eval "$(hydra-cli completion bash)"` (or sourced
// from a completions directory) wires it up. The subcommand list here is
// the SAME real one run()'s own switch in main.go dispatches on - kept
// as one small, explicit slice (not reflection over the switch, which Go
// has no clean way to enumerate) so adding a real subcommand to main.go
// without also listing it here is a one-line diff to catch in review,
// not a silently-stale completion script.
package main

import (
	"fmt"
	"io"
)

// topLevelCommands mirrors run()'s own case labels in main.go, minus the
// flag aliases (-v/--version, -h/--help) a real user tab-completing a
// bare word would not expect to see suggested.
var topLevelCommands = []string{"version", "help", "status", "robots", "claims", "claim", "release", "limits", "jog-arm", "jog", "table", "robot", "audit", "doctor", "config", "shell", "completion", "login", "logout", "scan", "fleet", "stop", "stop-all", "metrics", "manifest", "connections", "liveness", "ecosystem-health", "plan-simulate"}

// configSubcommands mirrors config.go's own real switch.
var configSubcommands = []string{"validate", "apply"}

// fleetSubcommands and manifestSubcommands mirror the switches of fleet.go and manifest.go.
var fleetSubcommands = []string{"doctor"}
var manifestSubcommands = []string{"validate"}

// commonOptions are read by resolveTarget or are shared by many commands.
var commonOptions = []string{"--server", "--config", "--timeout", "--json", "--help"}

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

func writeFishCompletion(w io.Writer) {
	fmt.Fprintf(w, `# fish completion for hydra-cli: hydra-cli completion fish | source
complete -c hydra-cli -f
complete -c hydra-cli -n "__fish_use_subcommand" -a "%s"
complete -c hydra-cli -n "__fish_seen_subcommand_from config" -a "%s"
complete -c hydra-cli -n "__fish_seen_subcommand_from fleet" -a "%s"
complete -c hydra-cli -n "__fish_seen_subcommand_from manifest" -a "%s"
`, joinWords(topLevelCommands), joinWords(configSubcommands), joinWords(fleetSubcommands), joinWords(manifestSubcommands))
	for _, option := range commonOptions {
		fmt.Fprintf(w, "complete -c hydra-cli -l %s\n", option[2:])
	}
}

func writePowerShellCompletion(w io.Writer) {
	fmt.Fprintf(w, `# PowerShell completion for hydra-cli: hydra-cli completion powershell | Out-String | Invoke-Expression
Register-ArgumentCompleter -Native -CommandName hydra-cli -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)
    $words = $commandAst.CommandElements | ForEach-Object { $_.ToString() }
    $candidates = if ($words.Count -le 1 -or ($words.Count -eq 2 -and $wordToComplete)) {
        '%s' -split ' '
    } elseif ($words[1] -eq 'config') {
        '%s' -split ' '
    } elseif ($words[1] -eq 'fleet') {
        '%s' -split ' '
    } elseif ($words[1] -eq 'manifest') {
        '%s' -split ' '
    } else {
        '%s' -split ' '
    }
    $candidates | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
    }
}
`, joinWords(topLevelCommands), joinWords(configSubcommands), joinWords(fleetSubcommands), joinWords(manifestSubcommands), joinWords(commonOptions))
}

func writeBashCompletion(w io.Writer) {
	fmt.Fprintf(w, `_hydra_cli_complete() {
    local cur prev
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    if [ "$COMP_CWORD" -eq 1 ]; then
        COMPREPLY=($(compgen -W "%s" -- "$cur"))
        return 0
    fi
    if [ "${COMP_WORDS[1]}" = "config" ] && [ "$COMP_CWORD" -eq 2 ]; then
        COMPREPLY=($(compgen -W "%s" -- "$cur"))
        return 0
    fi
    if [ "${COMP_WORDS[1]}" = "fleet" ] && [ "$COMP_CWORD" -eq 2 ]; then
        COMPREPLY=($(compgen -W "%s" -- "$cur"))
        return 0
    fi
    if [ "${COMP_WORDS[1]}" = "manifest" ] && [ "$COMP_CWORD" -eq 2 ]; then
        COMPREPLY=($(compgen -W "%s" -- "$cur"))
        return 0
    fi
    case "$cur" in
        -*) COMPREPLY=($(compgen -W "%s" -- "$cur")) ;;
    esac
}
complete -F _hydra_cli_complete hydra-cli
`, joinWords(topLevelCommands), joinWords(configSubcommands), joinWords(fleetSubcommands), joinWords(manifestSubcommands), joinWords(commonOptions))
}

func writeZshCompletion(w io.Writer) {
	fmt.Fprintf(w, `#compdef hydra-cli
_hydra_cli() {
    local -a top_level config_sub fleet_sub manifest_sub options
    top_level=(%s)
    config_sub=(%s)
    fleet_sub=(%s)
    manifest_sub=(%s)
    options=(%s)
    if (( CURRENT == 2 )); then
        compadd -a top_level
        return
    fi
    if [[ ${words[2]} == "config" && $CURRENT -eq 3 ]]; then
        compadd -a config_sub
        return
    fi
    if [[ ${words[2]} == "fleet" && $CURRENT -eq 3 ]]; then
        compadd -a fleet_sub
        return
    fi
    if [[ ${words[2]} == "manifest" && $CURRENT -eq 3 ]]; then
        compadd -a manifest_sub
        return
    fi
    if [[ ${words[CURRENT]} == -* ]]; then
        compadd -a options
    fi
}
compdef _hydra_cli hydra-cli
`, joinWords(topLevelCommands), joinWords(configSubcommands), joinWords(fleetSubcommands), joinWords(manifestSubcommands), joinWords(commonOptions))
}

func joinWords(words []string) string {
	out := ""
	for i, word := range words {
		if i > 0 {
			out += " "
		}
		out += word
	}
	return out
}
