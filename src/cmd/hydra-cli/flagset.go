// =============================================================================
// HYDRA-UMC-TOOL-CLI - Flag parsing on flag.FlagSet: cmd/hydra-cli/flagset.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0 - see LICENSE
// =============================================================================
// Every command declares the options it owns once (their names, whether they take a value) and the values are read by
// the standard library's flag.FlagSet. What stays exactly as the CLI has always behaved:
//   - options and positional arguments may be mixed in any order (`claim 3 --force` and `claim --force 3`);
//   - a token that is not one of the declared options is handed back untouched in Rest, so the command (or
//     rejectExtraArgs) reports it with the message it always had, instead of the flag package's own wording;
//   - a value option takes the next argument whatever it looks like, and a missing value is a usage error;
//   - the last occurrence of an option wins.
package main

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

// parsedFlags is the result of parseFlagSet.
type parsedFlags struct {
	set  *flag.FlagSet
	vals map[string]*string
	bits map[string]*bool
	// Rest is every argument that was not a declared option or its value, in the order given.
	Rest []string
}

// String is the value of a declared value option ("" when it was not given).
func (p *parsedFlags) String(name string) string {
	if v, ok := p.vals[name]; ok {
		return *v
	}
	return ""
}

// Bool tells whether a declared switch was given.
func (p *parsedFlags) Bool(name string) bool {
	if v, ok := p.bits[name]; ok {
		return *v
	}
	return false
}

// Seen tells whether an option (value or switch) appeared on the command line, even with an empty value.
func (p *parsedFlags) Seen(name string) bool {
	seen := false
	p.set.Visit(func(f *flag.Flag) {
		if f.Name == strings.TrimPrefix(name, "--") {
			seen = true
		}
	})
	return seen
}

// flagOpt is one declared option; the order in which a command lists them is the order they are taken out of the arguments.
type flagOpt struct {
	name  string
	value bool
}

// valueOpt declares an option that takes the next argument as its value.
func valueOpt(name string) flagOpt { return flagOpt{name: name, value: true} }

// switchOpt declares a bare option.
func switchOpt(name string) flagOpt { return flagOpt{name: name} }

func newParsedFlags(command string, opts []flagOpt) *parsedFlags {
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	p := &parsedFlags{set: fs, vals: map[string]*string{}, bits: map[string]*bool{}}
	for _, o := range opts {
		if o.value {
			p.vals[o.name] = fs.String(strings.TrimPrefix(o.name, "--"), "", "")
		} else {
			p.bits[o.name] = fs.Bool(strings.TrimPrefix(o.name, "--"), false, "")
		}
	}
	return p
}

func (p *parsedFlags) parseOwn(command string, own []string) error {
	if err := p.set.Parse(own); err != nil {
		return newCliError(ExitUsageError, fmt.Errorf("%s: %w", command, err))
	}
	return nil
}

// parseFlagSet reads the declared options out of args in one left-to-right pass. values and switches are option names with
// their leading "--" (for example "--reason"); command only names the flag set in errors.
func parseFlagSet(command string, args, values, switches []string) (*parsedFlags, error) {
	var opts []flagOpt
	isValue := map[string]bool{}
	isSwitch := map[string]bool{}
	for _, n := range values {
		opts = append(opts, valueOpt(n))
		isValue[n] = true
	}
	for _, n := range switches {
		opts = append(opts, switchOpt(n))
		isSwitch[n] = true
	}
	p := newParsedFlags(command, opts)

	// the declared options go to the FlagSet in its own `--name=value` form, so a value that starts with "-" stays a value
	var own []string
	p.Rest = make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch {
		case isValue[args[i]]:
			if i+1 >= len(args) {
				return nil, newCliError(ExitUsageError, fmt.Errorf("%s needs a value", args[i]))
			}
			own = append(own, args[i]+"="+args[i+1])
			i++
		case isSwitch[args[i]]:
			own = append(own, args[i])
		default:
			p.Rest = append(p.Rest, args[i])
		}
	}
	if err := p.parseOwn(command, own); err != nil {
		return nil, err
	}
	return p, nil
}

// parseFlagSetOrdered reads the declared options out of args one option at a time, in the order they are listed: the
// arguments an earlier option takes (its value included) are no longer seen by the later ones, so a value that looks like a
// later option stays the value of the earlier one. The commands that were first written with take-one-by-one parsing use it
// to keep reading `--reason --json` exactly as they always have.
func parseFlagSetOrdered(command string, args []string, opts ...flagOpt) (*parsedFlags, error) {
	p := newParsedFlags(command, opts)
	rest := args
	var own []string
	for _, o := range opts {
		next := make([]string, 0, len(rest))
		for i := 0; i < len(rest); i++ {
			switch {
			case rest[i] != o.name:
				next = append(next, rest[i])
			case !o.value:
				own = append(own, rest[i])
			case i+1 >= len(rest):
				return nil, newCliError(ExitUsageError, fmt.Errorf("%s needs a value", o.name))
			default:
				own = append(own, rest[i]+"="+rest[i+1])
				i++
			}
		}
		rest = next
	}
	if err := p.parseOwn(command, own); err != nil {
		return nil, err
	}
	p.Rest = rest
	return p, nil
}
