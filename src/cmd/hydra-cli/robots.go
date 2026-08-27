// =============================================================================
// HYDRA-UMC-TOOL-CLI - `robots` subcommand: cmd/hydra-cli/robots.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0 - see LICENSE
// =============================================================================
// The real fleet roster read, the first genuinely "fleet DevOps" command
// this CLI has (the README's own bigger promises - deploy/flash-all/
// audit - still need real write endpoints on HYDRA-UMC-SERVER that don't
// exist yet, see mejoras_futuras.txt). GET /api/settings already carries
// the full controller/robot roster and is a real, unauthenticated read
// (see HYDRA-UMC-SERVER/src/server.ts's own `app.get("/api/settings", ...)`
// route) - this command is a real client of that real, already-shipping
// contract, not new server-side work.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
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
	server, _ := resolveServer(args)

	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(server + "/api/settings")
	if err != nil {
		return fmt.Errorf("could not reach %s: %w", server, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response from %s: %w", server, err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s replied with HTTP %d: %s", server, resp.StatusCode, string(body))
	}

	var settings settingsResponse
	if err := json.Unmarshal(body, &settings); err != nil {
		return fmt.Errorf("unexpected response from %s: %w", server, err)
	}

	printRobotRoster(w, server, settings)
	return nil
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

	fmt.Fprintf(w, "%-14s %-20s %-8s %-24s %s\n", "CONTROLLER", "ROBOT", "ONLINE", "MODEL", "ROLE")
	for _, c := range settings.Controllers {
		for _, r := range c.Robots {
			online := "no"
			if r.Online {
				online = "yes"
			}
			fmt.Fprintf(w, "%-14s %-20s %-8s %-24s %s\n", c.Name, r.Name, online, r.Model, r.Role)
		}
	}
}
