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
var topLevelCommands = []string{"version", "help", "status", "robots", "doctor", "config", "shell", "completion"}

// configSubcommands mirrors config.go's own real switch.
var configSubcommands = []string{"validate", "apply"}

func cmdCompletion(w io.Writer, args []string) error {
	if len(args) != 1 {
		return newCliError(ExitUsageError, fmt.Errorf("usage: hydra-cli completion <bash|zsh>"))
	}
	switch args[0] {
	case "bash":
		writeBashCompletion(w)
		return nil
	case "zsh":
		writeZshCompletion(w)
		return nil
	default:
		return newCliError(ExitUsageError, fmt.Errorf("unsupported shell %q - expected \"bash\" or \"zsh\"", args[0]))
	}
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
}
complete -F _hydra_cli_complete hydra-cli
`, joinWords(topLevelCommands), joinWords(configSubcommands))
}

func writeZshCompletion(w io.Writer) {
	fmt.Fprintf(w, `#compdef hydra-cli
_hydra_cli() {
    local -a top_level config_sub
    top_level=(%s)
    config_sub=(%s)
    if (( CURRENT == 2 )); then
        compadd -a top_level
        return
    fi
    if [[ ${words[2]} == "config" && $CURRENT -eq 3 ]]; then
        compadd -a config_sub
        return
    fi
}
compdef _hydra_cli hydra-cli
`, joinWords(topLevelCommands), joinWords(configSubcommands))
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
