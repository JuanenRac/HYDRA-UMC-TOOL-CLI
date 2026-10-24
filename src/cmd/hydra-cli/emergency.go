// =============================================================================
// HYDRA-UMC-TOOL-CLI - Stopping robots: cmd/hydra-cli/emergency.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0-or-later - see LICENSE
// =============================================================================
// `stop <robot>` is the stop command of the server (POST /api/robot/:id/command with "stop"), which the
// server lets through even when another account reserved the robot: a stop is never blocked by a
// reservation. `stop-all` is the global stop of the server (POST /api/stop-all): every robot, whoever
// reserved it, and every valve and pump off. Because it reaches robots of other people it asks for
// --yes; `stop` is the safe direction for one robot and asks for nothing.
//
// Neither is an emergency stop in the sense of a safety standard: both are software commands that need
// the server, the network and a login. The physical E-STOP of the cell is what stops a machine.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"text/tabwriter"
)

// cmdStop implements `hydra-cli stop <robot> [--controller-id ID]`.
func cmdStop(w io.Writer, args []string) error {
	server, timeout, controllerID, rest, err := parseControlArgs(args)
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return newCliError(ExitUsageError, errors.New("usage: hydra-cli stop <robotId> [--controller-id ID]"))
	}
	robotID, err := parseRobotID(rest[0])
	if err != nil {
		return err
	}
	client := newClient(timeout)
	resp, err := sendAuthenticated(&client, server, http.MethodPost, fmt.Sprintf("/api/robot/%d/command", robotID),
		map[string]any{"command": "stop", "controllerId": controllerID}, controllerID)
	if err != nil {
		return err
	}
	if !okStatus(resp.Status) {
		return refusalError("Stop", resp)
	}
	fmt.Fprintf(w, "Stop sent to robot %d.\n", robotID)
	printRevision(w, resp)
	return nil
}

// stopAllRobot is one robot in the answer of POST /api/stop-all.
type stopAllRobot struct {
	RobotID    int    `json:"robotId"`
	Name       string `json:"name"`
	Stopped    bool   `json:"stopped"`
	ValvesOff  int    `json:"valvesOff"`
	PumpsOff   int    `json:"pumpsOff"`
	ReservedBy string `json:"reservedBy"`
	Error      string `json:"error"`
}

// cmdStopAll implements `hydra-cli stop-all --yes [--json]`.
func cmdStopAll(w io.Writer, args []string) error {
	server, timeout, controllerID, rest, err := parseControlArgs(args)
	if err != nil {
		return err
	}
	confirmed, rest := takeSwitch(rest, "--yes")
	jsonOutput, rest := takeSwitch(rest, "--json")
	if err := rejectExtraArgs("stop-all", rest); err != nil {
		return err
	}
	if !confirmed {
		return newCliError(ExitUsageError, errors.New("stop-all stops EVERY robot of the server, whoever reserved it, and switches every valve and pump off: add --yes to confirm (nothing was sent)"))
	}
	client := newClient(timeout)
	resp, err := sendAuthenticated(&client, server, http.MethodPost, "/api/stop-all", map[string]any{}, controllerID)
	if err != nil {
		return err
	}
	if !okStatus(resp.Status) {
		return refusalError("Stopping every robot", resp)
	}
	var answer struct {
		Success      bool           `json:"success"`
		StoppedCount int            `json:"stoppedCount"`
		FailedCount  int            `json:"failedCount"`
		Persisted    bool           `json:"persisted"`
		Robots       []stopAllRobot `json:"robots"`
	}
	if err := json.Unmarshal(resp.Body, &answer); err != nil {
		return newCliError(ExitServerError, fmt.Errorf("unexpected answer to the global stop: %w", err))
	}
	if jsonOutput {
		fmt.Fprintln(w, string(resp.Body))
	} else {
		fmt.Fprintf(w, "Global stop: %d robot(s) stopped, %d failed.\n", answer.StoppedCount, answer.FailedCount)
		if len(answer.Robots) > 0 {
			table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
			fmt.Fprintln(table, "ROBOT\tNAME\tSTOPPED\tVALVES OFF\tPUMPS OFF\tRESERVED BY")
			for _, robot := range answer.Robots {
				stopped := "yes"
				if !robot.Stopped {
					stopped = "NO " + printable(robot.Error)
				}
				fmt.Fprintf(table, "%d\t%s\t%s\t%d\t%d\t%s\n", robot.RobotID, printable(robot.Name), stopped, robot.ValvesOff, robot.PumpsOff, dash(printable(robot.ReservedBy)))
			}
			if err := table.Flush(); err != nil {
				return err
			}
		}
		if !answer.Persisted {
			fmt.Fprintln(w, "Warning: the settings could not be saved; the stop is applied in memory and was broadcast.")
		}
	}
	if answer.FailedCount > 0 || !answer.Success {
		return newCliError(ExitServerError, fmt.Errorf("%d robot(s) could not be stopped", answer.FailedCount))
	}
	return nil
}
