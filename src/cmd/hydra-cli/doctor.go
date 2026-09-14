// =============================================================================
// HYDRA-UMC-TOOL-CLI - Read-only HYDRA-UMC-SERVER diagnostic: cmd/hydra-cli/doctor.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0 - see LICENSE
// =============================================================================
// `hydra-cli doctor` proves that the two public read contracts this CLI already
// consumes agree with one another. It deliberately uses GET only: it neither
// commands robots nor probes CAN, cameras, sensors, or any other hardware.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// doctorInfo is the published, read-only subset of /api/hydra-info needed for
// a diagnosis. The count fields are pointers so an older server that does not
// report them remains usable, but the result explicitly says that cross-check
// evidence is unavailable rather than pretending it passed.
type doctorInfo struct {
	SchemaVersion    string `json:"schema_version"`
	Product          string `json:"product"`
	AppVersion       string `json:"appVersion"`
	Hostname         string `json:"hostname"`
	RemoteAPIVersion int    `json:"remoteApiVersion"`
	ControllerCount  *int   `json:"controllerCount"`
	RobotCount       *int   `json:"robotCount"`
}

// doctorCheck is one machine-readable diagnostic result inside --json
// output. checkId is a stable identifier a script can key off without
// parsing prose; severity classifies how much a "fail" status should matter
// to an automated caller: "critical" means doctor could not even complete
// (unreachable server, malformed response), while "warning" is the same
// best-effort cross-check the plain-text countCrossCheck=not-reported output
// already tolerates on an older server.
type doctorCheck struct {
	CheckID  string `json:"checkId"`
	Severity string `json:"severity"`
	Status   string `json:"status"` // "pass", "fail", or "not-reported"
	Message  string `json:"message,omitempty"`
}

// doctorReport is cmdDoctor's --json payload: the same facts the plain-text
// DOCTOR=PASS line already reports, plus the individual checks list that
// plain-text output has never exposed to a script. Checks accumulate in the
// order they run, so a report from a failed diagnosis still shows every
// check that passed before the one that failed.
type doctorReport struct {
	Server           string        `json:"server"`
	OK               bool          `json:"ok"`
	AppVersion       string        `json:"appVersion,omitempty"`
	SchemaVersion    string        `json:"schemaVersion,omitempty"`
	RemoteAPIVersion int           `json:"remoteApiVersion,omitempty"`
	Controllers      int           `json:"controllers,omitempty"`
	Robots           int           `json:"robots,omitempty"`
	Checks           []doctorCheck `json:"checks"`
}

// cmdDoctor implements a safe, endpoint-contract diagnostic. A successful
// diagnosis means only that the server answered valid JSON and that its
// published fleet counts agree with its settings response. It is not a claim
// about physical controller, actuator, or safety health.
//
// --json switches the plain-text DOCTOR=PASS line for a structured
// doctorReport with a checkId/severity/status per check, for a caller that
// wants to act on individual results instead of grepping prose.
func cmdDoctor(w io.Writer, args []string) error {
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
	if len(filtered) != 0 {
		return newCliError(ExitUsageError, fmt.Errorf("doctor does not accept arguments: %s", strings.Join(filtered, " ")))
	}

	report := doctorReport{Server: server}
	client := http.Client{Timeout: timeout}

	var info doctorInfo
	if err := getJSON(&client, server, "/api/hydra-info", &info); err != nil {
		report.Checks = append(report.Checks, doctorCheck{CheckID: "hydra-info-reachable", Severity: "critical", Status: "fail", Message: err.Error()})
		return finishDoctor(w, jsonOutput, report, err)
	}
	report.Checks = append(report.Checks, doctorCheck{CheckID: "hydra-info-reachable", Severity: "critical", Status: "pass"})

	if strings.TrimSpace(info.AppVersion) == "" {
		err := newCliError(ExitServerError, fmt.Errorf("%s returned /api/hydra-info without appVersion", server))
		report.Checks = append(report.Checks, doctorCheck{CheckID: "hydra-info-has-app-version", Severity: "critical", Status: "fail", Message: err.Error()})
		return finishDoctor(w, jsonOutput, report, err)
	}
	report.AppVersion = info.AppVersion
	report.SchemaVersion = info.SchemaVersion
	report.RemoteAPIVersion = info.RemoteAPIVersion
	report.Checks = append(report.Checks, doctorCheck{CheckID: "hydra-info-has-app-version", Severity: "critical", Status: "pass"})

	var settings settingsResponse
	if err := getJSON(&client, server, "/api/settings", &settings); err != nil {
		report.Checks = append(report.Checks, doctorCheck{CheckID: "settings-reachable", Severity: "critical", Status: "fail", Message: err.Error()})
		return finishDoctor(w, jsonOutput, report, err)
	}
	report.Checks = append(report.Checks, doctorCheck{CheckID: "settings-reachable", Severity: "critical", Status: "pass"})

	actualControllers := len(settings.Controllers)
	actualRobots := robotCount(settings)
	report.Controllers = actualControllers
	report.Robots = actualRobots

	if info.ControllerCount != nil && *info.ControllerCount != actualControllers {
		err := newCliError(ExitServerError, fmt.Errorf("controller count mismatch: /api/hydra-info reports %d but /api/settings contains %d", *info.ControllerCount, actualControllers))
		report.Checks = append(report.Checks, doctorCheck{CheckID: "controller-count-cross-check", Severity: "warning", Status: "fail", Message: err.Error()})
		return finishDoctor(w, jsonOutput, report, err)
	}
	if info.RobotCount != nil && *info.RobotCount != actualRobots {
		err := newCliError(ExitServerError, fmt.Errorf("robot count mismatch: /api/hydra-info reports %d but /api/settings contains %d", *info.RobotCount, actualRobots))
		report.Checks = append(report.Checks, doctorCheck{CheckID: "robot-count-cross-check", Severity: "warning", Status: "fail", Message: err.Error()})
		return finishDoctor(w, jsonOutput, report, err)
	}

	crossCheckStatus := "pass"
	if info.ControllerCount == nil || info.RobotCount == nil {
		crossCheckStatus = "not-reported"
	}
	report.Checks = append(report.Checks,
		doctorCheck{CheckID: "controller-count-cross-check", Severity: "warning", Status: crossCheckStatus},
		doctorCheck{CheckID: "robot-count-cross-check", Severity: "warning", Status: crossCheckStatus},
	)
	report.OK = true

	if jsonOutput {
		return writeDoctorJSON(w, report)
	}

	fmt.Fprintf(w, "DOCTOR=PASS server=%s appVersion=%s", server, info.AppVersion)
	if info.SchemaVersion != "" {
		fmt.Fprintf(w, " schema=%s", info.SchemaVersion)
	}
	if info.RemoteAPIVersion != 0 {
		fmt.Fprintf(w, " remoteApiVersion=%d", info.RemoteAPIVersion)
	}
	fmt.Fprintf(w, " controllers=%d robots=%d", actualControllers, actualRobots)
	fmt.Fprintf(w, " countCrossCheck=%s", crossCheckStatus)
	fmt.Fprintln(w)
	return nil
}

// finishDoctor centralizes the JSON-vs-plain-text choice on every early
// failure path. A --json caller gets the same structured report (marked
// ok=false, carrying whatever checks ran before the failure) that a healthy
// run produces, instead of plain-text mode's silence on stdout when it
// fails; a non-JSON caller's behavior is completely unchanged. Either way
// the original error is returned unmodified, so main's exit-code
// classification and stderr message stay exactly as before this flag
// existed.
func finishDoctor(w io.Writer, jsonOutput bool, report doctorReport, err error) error {
	if !jsonOutput {
		return err
	}
	report.OK = false
	if writeErr := writeDoctorJSON(w, report); writeErr != nil {
		return writeErr
	}
	return err
}

func writeDoctorJSON(w io.Writer, report doctorReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// getJSON owns the common read-only HTTP/error mapping used by doctor. Keeping
// this local prevents a broad refactor of established status/robots behaviour
// while ensuring both doctor endpoints receive identical failure handling.
func getJSON(client *http.Client, server, path string, target any) error {
	resp, err := client.Get(strings.TrimRight(server, "/") + path)
	if err != nil {
		return newCliError(ExitNetworkError, fmt.Errorf("could not reach %s: %w", server, err))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return newCliError(ExitNetworkError, fmt.Errorf("reading response from %s: %w", server, err))
	}
	if resp.StatusCode != http.StatusOK {
		return newCliError(ExitServerError, fmt.Errorf("%s%s replied with HTTP %d: %s", server, path, resp.StatusCode, string(body)))
	}
	if err := json.Unmarshal(body, target); err != nil {
		return newCliError(ExitServerError, fmt.Errorf("unexpected response from %s%s: %w", server, path, err))
	}
	return nil
}

func robotCount(settings settingsResponse) int {
	total := 0
	for _, controller := range settings.Controllers {
		total += len(controller.Robots)
	}
	return total
}
