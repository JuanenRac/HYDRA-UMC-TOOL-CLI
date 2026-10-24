// =============================================================================
// HYDRA-UMC-TOOL-CLI - `robots` subcommand: cmd/hydra-cli/robots.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0 - see LICENSE
// =============================================================================
// The real fleet roster read, the first genuinely "fleet DevOps" command
// this CLI has (the README's own bigger promises - deploy/flash-all/
// audit - still need real write endpoints on HYDRA-UMC-SERVER that don't
// exist yet). GET /api/settings already carries the full
// controller/robot roster; the server gives it only to a logged-in account
// (see auth.go) - this command is a real client of that already-shipping
// contract, not new server-side work.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"text/tabwriter"
)

// robotEntry mirrors the subset of one robot object inside
// GET /api/settings's own controllers[].robots array that this command
// actually prints. Matches HYDRA-UMC-SERVER's own data/settings.json
// shape - unknown fields (pos, joints, valves, ...) are ignored, so this
// stays forward-compatible with a server that adds fields later.
//
// Deliberately has no `id` field: a real end-to-end smoke test against a
// real running HYDRA-UMC-SERVER (not just the example data/settings.json
// on disk) found that a real controller's own `id` is a string
// ("localhost" for the default local controller) while a real robot's
// own `id` is a number - genuine API inconsistency, not a typo. Since
// printRobotRoster never actually needed either id, dropping both here
// sidesteps that inconsistency entirely instead of fighting it with a
// custom UnmarshalJSON just to discard the value anyway.
type robotEntry struct {
	Name   string `json:"name"`
	Online bool   `json:"online"`
	Model  string `json:"model"`
	Role   string `json:"role"`
}

// controllerEntry mirrors one controller object inside
// GET /api/settings's own top-level controllers array.
type controllerEntry struct {
	Name   string       `json:"name"`
	IP     string       `json:"ip"`
	Status string       `json:"status"`
	Robots []robotEntry `json:"robots"`
}

// settingsResponse mirrors the subset of GET /api/settings's own
// top-level response this command reads.
type settingsResponse struct {
	Controllers []controllerEntry `json:"controllers"`
}

// cmdRobots implements `hydra-cli robots` - a real HTTP GET against a
// live HYDRA-UMC-SERVER's own /api/settings, printing the real
// controller/robot roster it reports. Real, not a stub: every field
// printed comes straight from the live response, nothing hardcoded.
// Takes an explicit io.Writer (main.go passes os.Stdout) rather than
// hardcoding os.Stdout itself, so tests can assert on the real printed
// output via a bytes.Buffer instead of redirecting the process's real
// stdout.
func cmdRobots(w io.Writer, args []string) error {
	server, timeout, rest, err := resolveTarget(args)
	if err != nil {
		return err
	}
	jsonOutput := false
	filtered := make([]string, 0, len(rest))
	for _, arg := range rest {
		if arg == "--json" {
			jsonOutput = true
			continue
		}
		filtered = append(filtered, arg)
	}
	if err := rejectExtraArgs("robots", filtered); err != nil {
		return err
	}

	client := newClient(timeout)
	status, body, err := getAuthenticated(&client, server, "/api/settings")
	if err != nil {
		return err
	}

	if status != http.StatusOK {
		return newCliError(ExitServerError, fmt.Errorf("%s replied with HTTP %d: %s", server, status, printable(string(body))))
	}

	var settings settingsResponse
	if err := json.Unmarshal(body, &settings); err != nil {
		return newCliError(ExitServerError, fmt.Errorf("unexpected response from %s: %w", server, err))
	}

	if jsonOutput {
		return printRobotRosterJSON(w, settings)
	}
	printRobotRoster(w, server, settings)
	return nil
}

// robotRow is one line of the machine-readable roster (`robots --json`).
type robotRow struct {
	Controller string `json:"controller"`
	Robot      string `json:"robot"`
	Online     bool   `json:"online"`
	Model      string `json:"model"`
	Role       string `json:"role"`
}

// printRobotRosterJSON writes the roster as a JSON array - always an array,
// empty when the server reports no robots, so a script never has to guess.
func printRobotRosterJSON(w io.Writer, settings settingsResponse) error {
	rows := make([]robotRow, 0)
	for _, c := range settings.Controllers {
		for _, r := range c.Robots {
			rows = append(rows, robotRow{Controller: c.Name, Robot: r.Name, Online: r.Online, Model: r.Model, Role: r.Role})
		}
	}
	encoded, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return newCliError(ExitGeneralError, fmt.Errorf("could not encode the roster: %w", err))
	}
	_, err = fmt.Fprintln(w, string(encoded))
	return err
}

// printRobotRoster does the actual formatting, split out from cmdRobots
// so tests can assert on the printed table without capturing os.Stdout.
func printRobotRoster(w io.Writer, server string, settings settingsResponse) {
	totalRobots := 0
	for _, c := range settings.Controllers {
		totalRobots += len(c.Robots)
	}

	if totalRobots == 0 {
		fmt.Fprintf(w, "no robots reported by %s\n", server)
		return
	}

	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "CONTROLLER\tROBOT\tONLINE\tMODEL\tROLE")
	for _, c := range settings.Controllers {
		for _, r := range c.Robots {
			online := "no"
			if r.Online {
				online = "yes"
			}
			fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\n", printable(c.Name), printable(r.Name), online, printable(r.Model), printable(r.Role))
		}
	}
	table.Flush()
}
