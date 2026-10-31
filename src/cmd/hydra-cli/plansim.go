// =============================================================================
// HYDRA-UMC-TOOL-CLI - Plan simulation: cmd/hydra-cli/plansim.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0-or-later - see LICENSE
// =============================================================================
// `hydra-cli plan-simulate --plan FILE` sends a plan to the dry run of HYDRA-UMC-SEMANTIC-PLANNER
// (POST /plan/simulate) and prints its verdict. The planner checks the plan against declared constraints on a
// simulated clock; it never executes, queues or releases anything, and neither does this command.
//
// The planner is another service than the server, with its own address (--planner URL, HYDRA_CLI_PLANNER, then
// http://localhost:8109), so the credential of the server is NEVER sent to it. The answer is checked before
// anything is printed: a verdict that is not accepted or rejected, one that claims the plan was executed or is
// executable, a violation without a rule, a timeline that does not have the shape of the contract: all are an
// error (exit 5), never an "accepted". A rejected plan is not an error of the service but a refusal by the
// rules of the plan: it exits 8 after printing every broken rule.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
)

const (
	plannerEnv        = "HYDRA_CLI_PLANNER"
	defaultPlannerURL = "http://localhost:8109"
	// maxPlanBytes caps the plan file: the planner itself allows at most 10000 steps.
	maxPlanBytes = 4 << 20
)

type simViolation struct {
	Rule       string `json:"rule"`
	StepIndex  *int   `json:"step_index,omitempty"`
	StepID     string `json:"step_id,omitempty"`
	Message    string `json:"message"`
	Value      any    `json:"value,omitempty"`
	Limit      any    `json:"limit,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`
}

type simTimelineEntry struct {
	StepID    string `json:"step_id"`
	Index     *int   `json:"index"`
	Robot     string `json:"robot"`
	Primitive string `json:"primitive"`
	StartMs   *int64 `json:"start_ms"`
	EndMs     *int64 `json:"end_ms"`
}

type simVerdict struct {
	Verdict          string             `json:"verdict"`
	Executable       *bool              `json:"executable"`
	Executed         *bool              `json:"executed"`
	Violations       []simViolation     `json:"violations"`
	Timeline         []simTimelineEntry `json:"timeline"`
	MakespanMs       *int64             `json:"makespan_ms"`
	SimulatedClockMs *int64             `json:"simulated_clock_ms"`
}

func validateVerdict(v simVerdict) error {
	switch {
	case v.Verdict != "accepted" && v.Verdict != "rejected":
		return fmt.Errorf("the verdict is %q, not accepted or rejected", v.Verdict)
	case v.Executable == nil || *v.Executable:
		return errors.New("the answer does not say executable=false")
	case v.Executed == nil || *v.Executed:
		return errors.New("the answer does not say executed=false")
	case v.Violations == nil || v.Timeline == nil:
		return errors.New("the violations list or the timeline is missing")
	case v.MakespanMs == nil || *v.MakespanMs < 0 || v.SimulatedClockMs == nil || *v.SimulatedClockMs < 0:
		return errors.New("makespan_ms or simulated_clock_ms is missing or negative")
	case v.Verdict == "accepted" && len(v.Violations) > 0:
		return errors.New("the verdict is accepted but violations are listed")
	case v.Verdict == "rejected" && len(v.Violations) == 0:
		return errors.New("the verdict is rejected but no violation is listed")
	}
	for i, violation := range v.Violations {
		if violation.Rule == "" || violation.Message == "" {
			return fmt.Errorf("violation %d has no rule or no message", i)
		}
	}
	for i, entry := range v.Timeline {
		if entry.StepID == "" || entry.Primitive == "" || entry.Index == nil || entry.StartMs == nil || entry.EndMs == nil || *entry.StartMs < 0 || *entry.EndMs < *entry.StartMs {
			return fmt.Errorf("timeline entry %d is incomplete or ends before it starts", i)
		}
	}
	return nil
}

// readPlan reads the plan file ("-" is the standard input) and checks that it is one JSON object.
func readPlan(path string, in io.Reader) ([]byte, error) {
	var reader io.Reader = in
	if path != "-" {
		file, err := os.Open(path)
		if err != nil {
			return nil, newCliError(ExitUsageError, fmt.Errorf("the plan file cannot be read: %w", err))
		}
		defer file.Close()
		reader = file
	}
	raw, err := io.ReadAll(io.LimitReader(reader, maxPlanBytes+1))
	if err != nil {
		return nil, newCliError(ExitUsageError, fmt.Errorf("the plan cannot be read: %w", err))
	}
	if len(raw) > maxPlanBytes {
		return nil, newCliError(ExitUsageError, fmt.Errorf("the plan is larger than %d bytes", maxPlanBytes))
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return nil, newCliError(ExitUsageError, errors.New("the plan must be one JSON object with a \"steps\" list (see docs/PLAN_SIMULATION.md of the planner)"))
	}
	return raw, nil
}

// cmdPlanSimulate implements `hydra-cli plan-simulate --plan FILE|- [--planner URL] [--json]`.
func cmdPlanSimulate(w io.Writer, in io.Reader, args []string) error {
	for _, arg := range args {
		if arg == "--server" {
			return newCliError(ExitUsageError, errors.New("plan-simulate talks to the planner, not to the server: use --planner URL (or "+plannerEnv+")"))
		}
	}
	_, timeout, rest, err := resolveTarget(args)
	if err != nil {
		return err
	}
	jsonOutput, rest := takeSwitch(rest, "--json")
	planPath, rest, err := takeFlag(rest, "--plan")
	if err != nil {
		return err
	}
	planner, rest, err := takeFlag(rest, "--planner")
	if err != nil {
		return err
	}
	if err := rejectExtraArgs("plan-simulate", rest); err != nil {
		return err
	}
	if planPath == "" {
		return newCliError(ExitUsageError, errors.New("usage: hydra-cli plan-simulate --plan FILE|- [--planner URL] [--json]"))
	}
	if planner == "" {
		planner = strings.TrimSpace(os.Getenv(plannerEnv))
	}
	if planner == "" {
		planner = defaultPlannerURL
	}
	planner, err = normalizeServer(planner)
	if err != nil {
		return err
	}
	plan, err := readPlan(planPath, in)
	if err != nil {
		return err
	}

	client := newClient(timeout)
	// no credential: the planner is not the server and must never see the token of the server
	resp, err := client.Post(planner+"/plan/simulate", "application/json", bytes.NewReader(plan))
	if err != nil {
		return newCliError(ExitNetworkError, fmt.Errorf("could not reach the planner at %s: %w", planner, err))
	}
	defer resp.Body.Close()
	body, err := readBody(resp.Body)
	if err != nil {
		return bodyReadError(planner, "response", err)
	}
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return newCliError(ExitServerError, fmt.Errorf("%s does not offer /plan/simulate (HTTP 404): switch plan_simulation.enabled on in the configuration of the planner", planner))
	case http.StatusBadRequest:
		return newCliError(ExitUsageError, fmt.Errorf("the planner could not understand the plan: %s", plannerReason(body)))
	default:
		return newCliError(ExitServerError, fmt.Errorf("%s replied with HTTP %d: %s", planner, resp.StatusCode, plannerReason(body)))
	}
	var verdict simVerdict
	if err := json.Unmarshal(body, &verdict); err != nil {
		return malformedAnswer(planner, "/plan/simulate", err.Error())
	}
	if err := validateVerdict(verdict); err != nil {
		return malformedAnswer(planner, "/plan/simulate", err.Error())
	}
	accepted := verdict.Verdict == "accepted"
	if jsonOutput {
		clean, err := cleanForOutput(verdict)
		if err != nil {
			return err
		}
		fields, _ := clean.(map[string]any)
		if err := writeJSONResult(w, "plan-simulate", accepted, fields); err != nil {
			return err
		}
	} else {
		printVerdict(w, verdict)
	}
	if !accepted {
		return newCliError(ExitRefused, fmt.Errorf("the plan was rejected by the simulation (%d rule(s) broken); nothing was executed", len(verdict.Violations)))
	}
	return nil
}

func plannerReason(body []byte) string {
	var parsed struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &parsed) == nil && parsed.Error != "" {
		return printable(parsed.Error)
	}
	return printable(string(body))
}

func printVerdict(w io.Writer, v simVerdict) {
	fmt.Fprintf(w, "Plan %s by the simulation (nothing was executed). Makespan %d ms, simulated clock %d ms.\n", strings.ToUpper(v.Verdict), *v.MakespanMs, *v.SimulatedClockMs)
	for _, violation := range v.Violations {
		step := "-"
		if violation.StepID != "" {
			step = printable(violation.StepID)
		}
		fmt.Fprintf(w, "  broken: %s at step %s: %s\n", printable(violation.Rule), step, printable(violation.Message))
		if violation.Suggestion != "" {
			fmt.Fprintf(w, "    suggestion: %s\n", printable(violation.Suggestion))
		}
	}
	if len(v.Timeline) > 0 {
		table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintln(table, "STEP\tROBOT\tPRIMITIVE\tSTART (ms)\tEND (ms)")
		for _, entry := range v.Timeline {
			fmt.Fprintf(table, "%s\t%s\t%s\t%d\t%d\n", printable(entry.StepID), printable(entry.Robot), printable(entry.Primitive), *entry.StartMs, *entry.EndMs)
		}
		table.Flush()
	}
}
