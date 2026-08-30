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
	"time"
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

// cmdDoctor implements a safe, endpoint-contract diagnostic. A successful
// diagnosis means only that the server answered valid JSON and that its
// published fleet counts agree with its settings response. It is not a claim
// about physical controller, actuator, or safety health.
func cmdDoctor(w io.Writer, args []string) error {
	server, rest := resolveServer(args)
	if len(rest) != 0 {
		return newCliError(ExitUsageError, fmt.Errorf("doctor does not accept arguments: %s", strings.Join(rest, " ")))
	}

	client := http.Client{Timeout: 5 * time.Second}
	var info doctorInfo
	if err := getJSON(&client, server, "/api/hydra-info", &info); err != nil {
		return err
	}
	if strings.TrimSpace(info.AppVersion) == "" {
		return newCliError(ExitServerError, fmt.Errorf("%s returned /api/hydra-info without appVersion", server))
	}

	var settings settingsResponse
	if err := getJSON(&client, server, "/api/settings", &settings); err != nil {
		return err
	}

	actualControllers := len(settings.Controllers)
	actualRobots := robotCount(settings)
	if info.ControllerCount != nil && *info.ControllerCount != actualControllers {
		return newCliError(ExitServerError, fmt.Errorf("controller count mismatch: /api/hydra-info reports %d but /api/settings contains %d", *info.ControllerCount, actualControllers))
	}
	if info.RobotCount != nil && *info.RobotCount != actualRobots {
		return newCliError(ExitServerError, fmt.Errorf("robot count mismatch: /api/hydra-info reports %d but /api/settings contains %d", *info.RobotCount, actualRobots))
	}

	fmt.Fprintf(w, "DOCTOR=PASS server=%s appVersion=%s", server, info.AppVersion)
	if info.SchemaVersion != "" {
		fmt.Fprintf(w, " schema=%s", info.SchemaVersion)
	}
	if info.RemoteAPIVersion != 0 {
		fmt.Fprintf(w, " remoteApiVersion=%d", info.RemoteAPIVersion)
	}
	fmt.Fprintf(w, " controllers=%d robots=%d", actualControllers, actualRobots)
	if info.ControllerCount == nil || info.RobotCount == nil {
		fmt.Fprint(w, " countCrossCheck=not-reported")
	} else {
		fmt.Fprint(w, " countCrossCheck=pass")
	}
	fmt.Fprintln(w)
	return nil
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
