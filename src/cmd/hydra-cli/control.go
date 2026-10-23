// =============================================================================
// HYDRA-UMC-TOOL-CLI - Reservations, safety limits and jog: cmd/hydra-cli/control.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0 - see LICENSE
// =============================================================================
// The control side of HYDRA-UMC-SERVER that every client shares: who reserves
// which robot (GET /api/claims, claim, release), the safety limits the server
// holds (GET /api/safety-limits), the explicit arming of the jog and a jog
// itself. Every request carries the controller id of this CLI session, so a
// reservation made here belongs to this program and not to the whole account.
//
// The controller id defaults to a stable name derived from the host
// ("hydra-cli-<host>") so a reservation survives from one command to the next;
// --controller-id or HYDRA_CLI_CONTROLLER_ID replace it (a `shell` session
// keeps one for its whole life). The limits are enforced by the server; the
// CLI additionally refuses, before sending, a jog above the limit of the
// model, and never sends a jog when it cannot read the limit.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
)

const controllerEnv = "HYDRA_CLI_CONTROLLER_ID"

var controllerIDPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,64}$`)

// defaultControllerID names this CLI on this host with only the characters the
// server accepts in a controller id.
func defaultControllerID() string {
	host, _ := os.Hostname()
	clean := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			return r
		}
		return '-'
	}, host)
	if clean == "" {
		clean = "local"
	}
	id := "hydra-cli-" + clean
	if len(id) > 64 {
		id = id[:64]
	}
	return id
}

// resolveControllerID picks the controller id of this session: the flag, then
// the environment, then the default of the host. A malformed id is a usage error.
func resolveControllerID(flagValue string) (string, error) {
	id := strings.TrimSpace(flagValue)
	if id == "" {
		id = strings.TrimSpace(os.Getenv(controllerEnv))
	}
	if id == "" {
		id = defaultControllerID()
	}
	if !controllerIDPattern.MatchString(id) {
		return "", newCliError(ExitUsageError, fmt.Errorf("the controller id must be 1 to 64 characters: letters, digits, . _ : - (got %q)", id))
	}
	return id, nil
}

// takeFlag removes `--name value` from args and returns the value.
func takeFlag(args []string, name string) (string, []string, error) {
	rest := make([]string, 0, len(args))
	value := ""
	for i := 0; i < len(args); i++ {
		if args[i] == name {
			if i+1 >= len(args) {
				return "", nil, newCliError(ExitUsageError, fmt.Errorf("%s needs a value", name))
			}
			value = args[i+1]
			i++
			continue
		}
		rest = append(rest, args[i])
	}
	return value, rest, nil
}

// takeSwitch removes a bare `--name` from args.
func takeSwitch(args []string, name string) (bool, []string) {
	rest := make([]string, 0, len(args))
	found := false
	for _, a := range args {
		if a == name {
			found = true
			continue
		}
		rest = append(rest, a)
	}
	return found, rest
}

func parseRobotID(raw string) (int, error) {
	id, err := strconv.Atoi(raw)
	if err != nil || id < 0 {
		return 0, newCliError(ExitUsageError, fmt.Errorf("the robot id must be a whole number, got %q", raw))
	}
	return id, nil
}

// controlResponse is one answer of the server to a control request.
type controlResponse struct {
	Status int
	Body   []byte
}

// code returns the stable refusal code of an error answer ("" when none).
func (r controlResponse) code() string {
	var parsed struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(r.Body, &parsed)
	return parsed.Code
}

func (r controlResponse) detail() string {
	var parsed struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(r.Body, &parsed) == nil && parsed.Error != "" {
		return printable(parsed.Error)
	}
	return ""
}

// sendAuthenticated sends one request with the credential and the controller id.
func sendAuthenticated(client *http.Client, server, method, path string, payload any, controllerID string) (controlResponse, error) {
	token, err := resolveToken(client, server)
	if err != nil {
		return controlResponse{}, err
	}
	var reader io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return controlResponse{}, newCliError(ExitGeneralError, err)
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, strings.TrimRight(server, "/")+path, reader)
	if err != nil {
		return controlResponse{}, newCliError(ExitConfigError, fmt.Errorf("invalid server %q: %w", server, err))
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if controllerID != "" {
		req.Header.Set("X-Controller-Id", controllerID)
	}
	if token != "" {
		warnIfInsecure(server)
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return controlResponse{}, newCliError(ExitNetworkError, fmt.Errorf("could not reach %s: %w", server, err))
	}
	defer resp.Body.Close()
	body, err := readBody(resp.Body)
	if err != nil {
		return controlResponse{}, bodyReadError(server, "response", err)
	}
	return controlResponse{Status: resp.StatusCode, Body: body}, nil
}

// refusalError turns a non-2xx answer into an error with a message that says
// what to do. Reservation and arming refusals are exit code 8 (refused), apart
// from a server fault (5) and a missing login (7).
func refusalError(what string, r controlResponse) error {
	detail := r.detail()
	suffix := ""
	if detail != "" {
		suffix = " (" + detail + ")"
	}
	switch {
	case r.Status == http.StatusUnauthorized:
		return newCliError(ExitAuthError, fmt.Errorf("%s failed: the login was refused or the token expired; sign in again%s", what, suffix))
	case r.code() == "jog-not-armed":
		return newCliError(ExitRefused, fmt.Errorf("%s refused: the jog is not armed; run `hydra-cli jog-arm <robot> on` first%s", what, suffix))
	case r.code() == "controller-id-required" && r.Status == http.StatusBadRequest:
		return newCliError(ExitRefused, fmt.Errorf("%s refused: this server requires the request to name its controller; set --controller-id or %s%s", what, controllerEnv, suffix))
	case r.code() == "controller-id-required":
		return newCliError(ExitRefused, fmt.Errorf("%s refused: the reservation names a controller and this request named none; use --controller-id or %s with that id%s", what, controllerEnv, suffix))
	case r.code() == "other-controller":
		return newCliError(ExitRefused, fmt.Errorf("%s refused: the robot is reserved by your account from another window or device; use that one, or release the reservation there%s", what, suffix))
	case r.code() == "other-account":
		return newCliError(ExitRefused, fmt.Errorf("%s refused: the robot is reserved by another account%s", what, suffix))
	case r.Status == http.StatusConflict:
		return newCliError(ExitRefused, fmt.Errorf("%s refused: the robot is reserved%s", what, suffix))
	case r.Status == http.StatusForbidden:
		return newCliError(ExitAuthError, fmt.Errorf("%s refused: this account is not allowed to do that%s", what, suffix))
	case r.Status == http.StatusNotFound:
		return newCliError(ExitServerError, fmt.Errorf("%s failed: the server does not know this robot%s", what, suffix))
	case r.Status == http.StatusBadRequest:
		return newCliError(ExitRefused, fmt.Errorf("%s refused by the server%s", what, suffix))
	}
	return newCliError(ExitServerError, fmt.Errorf("%s failed: the server replied with HTTP %d%s", what, r.Status, suffix))
}

func okStatus(status int) bool { return status >= 200 && status < 300 }

// parseControlArgs resolves the target, the controller id and the credential
// context shared by every control command.
func parseControlArgs(args []string) (server string, timeout time.Duration, controllerID string, rest []string, err error) {
	server, timeout, rest, err = resolveTarget(args)
	if err != nil {
		return
	}
	var flagID string
	flagID, rest, err = takeFlag(rest, "--controller-id")
	if err != nil {
		return
	}
	controllerID, err = resolveControllerID(flagID)
	return
}

// ---- claims ---------------------------------------------------------------

type claimEntry struct {
	RobotID       int    `json:"robotId"`
	RobotName     string `json:"robotName"`
	OwnerUsername string `json:"ownerUsername"`
	ControllerID  string `json:"controllerId"`
	ExpiresAt     int64  `json:"expiresAt"`
}

// cmdClaims implements `hydra-cli claims [--json]`: who controls which robot.
func cmdClaims(w io.Writer, args []string) error {
	server, timeout, controllerID, rest, err := parseControlArgs(args)
	if err != nil {
		return err
	}
	jsonOutput, rest := takeSwitch(rest, "--json")
	if err := rejectExtraArgs("claims", rest); err != nil {
		return err
	}
	client := http.Client{Timeout: timeout}
	resp, err := sendAuthenticated(&client, server, http.MethodGet, "/api/claims", nil, controllerID)
	if err != nil {
		return err
	}
	if !okStatus(resp.Status) {
		return refusalError("Listing the reservations", resp)
	}
	var parsed struct {
		Claims []claimEntry `json:"claims"`
	}
	if err := json.Unmarshal(resp.Body, &parsed); err != nil {
		return newCliError(ExitServerError, fmt.Errorf("unexpected response from %s: %w", server, err))
	}
	if jsonOutput {
		if parsed.Claims == nil {
			parsed.Claims = []claimEntry{}
		}
		encoded, _ := json.MarshalIndent(parsed.Claims, "", "  ")
		_, err := fmt.Fprintln(w, string(encoded))
		return err
	}
	if len(parsed.Claims) == 0 {
		fmt.Fprintln(w, "No robot is reserved.")
		return nil
	}
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "ROBOT\tNAME\tHELD BY\tCONTROLLER\tEXPIRES IN")
	for _, c := range parsed.Claims {
		controller := "not named"
		if c.ControllerID != "" {
			controller = printable(c.ControllerID)
			if c.ControllerID == controllerID {
				controller += " (this program)"
			}
		}
		fmt.Fprintf(table, "%d\t%s\t%s\t%s\t%s\n", c.RobotID, printable(c.RobotName), printable(c.OwnerUsername), controller, remaining(c.ExpiresAt))
	}
	return table.Flush()
}

// remaining formats the time left until an expiry given in milliseconds since the epoch.
func remaining(expiresAtMs int64) string {
	left := time.Until(time.UnixMilli(expiresAtMs)).Round(time.Second)
	if left <= 0 {
		return "expired"
	}
	return left.String()
}

// cmdClaim implements `hydra-cli claim <robot> [--ttl-ms N] [--force] [--reason TEXT]`.
func cmdClaim(w io.Writer, args []string) error {
	server, timeout, controllerID, rest, err := parseControlArgs(args)
	if err != nil {
		return err
	}
	force, rest := takeSwitch(rest, "--force")
	reason, rest, err := takeFlag(rest, "--reason")
	if err != nil {
		return err
	}
	ttlRaw, rest, err := takeFlag(rest, "--ttl-ms")
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return newCliError(ExitUsageError, errors.New("usage: hydra-cli claim <robotId> [--ttl-ms N] [--force] [--reason TEXT] [--controller-id ID]"))
	}
	robotID, err := parseRobotID(rest[0])
	if err != nil {
		return err
	}
	body := map[string]any{"controllerId": controllerID}
	if ttlRaw != "" {
		ttl, convErr := strconv.Atoi(ttlRaw)
		if convErr != nil || ttl <= 0 {
			return newCliError(ExitUsageError, fmt.Errorf("--ttl-ms must be a positive whole number, got %q", ttlRaw))
		}
		body["ttlMs"] = ttl
	}
	if force {
		body["force"] = true
	}
	if reason != "" {
		body["reason"] = reason
	}
	client := http.Client{Timeout: timeout}
	resp, err := sendAuthenticated(&client, server, http.MethodPost, fmt.Sprintf("/api/robot/%d/claim", robotID), body, controllerID)
	if err != nil {
		return err
	}
	if !okStatus(resp.Status) {
		return refusalError("Reservation", resp)
	}
	fmt.Fprintf(w, "Robot %d is reserved for this program (controller %s).\n", robotID, controllerID)
	return nil
}

// cmdRelease implements `hydra-cli release <robot> [--reason TEXT]`.
func cmdRelease(w io.Writer, args []string) error {
	server, timeout, controllerID, rest, err := parseControlArgs(args)
	if err != nil {
		return err
	}
	reason, rest, err := takeFlag(rest, "--reason")
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return newCliError(ExitUsageError, errors.New("usage: hydra-cli release <robotId> [--reason TEXT] [--controller-id ID]"))
	}
	robotID, err := parseRobotID(rest[0])
	if err != nil {
		return err
	}
	body := map[string]any{"controllerId": controllerID}
	if reason != "" {
		body["reason"] = reason
	}
	client := http.Client{Timeout: timeout}
	resp, err := sendAuthenticated(&client, server, http.MethodPost, fmt.Sprintf("/api/robot/%d/release", robotID), body, controllerID)
	if err != nil {
		return err
	}
	if !okStatus(resp.Status) {
		return refusalError("Release", resp)
	}
	fmt.Fprintf(w, "Robot %d is released.\n", robotID)
	return nil
}

// ---- safety limits ----------------------------------------------------------

type jogLimits struct {
	MaxAmount   float64 `json:"maxAmount"`
	MaxAbsolute float64 `json:"maxAbsolute"`
}

type safetyView struct {
	Limits struct {
		Jog struct {
			ArmRequired bool      `json:"armRequired"`
			ArmTTLMs    int       `json:"armTtlMs"`
			Default     jogLimits `json:"default"`
		} `json:"jog"`
		Temperature struct {
			Default struct {
				BedMaxC float64 `json:"bedMaxC"`
			} `json:"default"`
		} `json:"temperature"`
		Claims struct {
			RequireControllerID bool `json:"requireControllerId"`
		} `json:"claims"`
	} `json:"limits"`
	Robot *struct {
		ID          int       `json:"id"`
		Model       string    `json:"model"`
		Jog         jogLimits `json:"jog"`
		Temperature struct {
			BedMaxC float64 `json:"bedMaxC"`
		} `json:"temperature"`
		JogArmedUntil *int64 `json:"jogArmedUntil"`
	} `json:"robot"`
}

func fetchSafety(client *http.Client, server, controllerID string, robotID *int) (safetyView, []byte, error) {
	path := "/api/safety-limits"
	if robotID != nil {
		path += "?robotId=" + url.QueryEscape(strconv.Itoa(*robotID))
	}
	resp, err := sendAuthenticated(client, server, http.MethodGet, path, nil, controllerID)
	if err != nil {
		return safetyView{}, nil, err
	}
	if !okStatus(resp.Status) {
		return safetyView{}, nil, refusalError("Reading the safety limits", resp)
	}
	var view safetyView
	if err := json.Unmarshal(resp.Body, &view); err != nil {
		return safetyView{}, nil, newCliError(ExitServerError, fmt.Errorf("unexpected response from %s: %w", server, err))
	}
	return view, resp.Body, nil
}

// cmdLimits implements `hydra-cli limits [--robot ID] [--json]`.
func cmdLimits(w io.Writer, args []string) error {
	server, timeout, controllerID, rest, err := parseControlArgs(args)
	if err != nil {
		return err
	}
	jsonOutput, rest := takeSwitch(rest, "--json")
	robotRaw, rest, err := takeFlag(rest, "--robot")
	if err != nil {
		return err
	}
	if err := rejectExtraArgs("limits", rest); err != nil {
		return err
	}
	var robotID *int
	if robotRaw != "" {
		id, err := parseRobotID(robotRaw)
		if err != nil {
			return err
		}
		robotID = &id
	}
	client := http.Client{Timeout: timeout}
	view, raw, err := fetchSafety(&client, server, controllerID, robotID)
	if err != nil {
		return err
	}
	if jsonOutput {
		var indented bytes.Buffer
		if json.Indent(&indented, raw, "", "  ") != nil {
			indented.Write(raw)
		}
		_, err := fmt.Fprintln(w, indented.String())
		return err
	}
	fmt.Fprintf(w, "Relative jog, at most:   %g (default for every model)\n", view.Limits.Jog.Default.MaxAmount)
	fmt.Fprintf(w, "Absolute jog, at most:   %g (default for every model)\n", view.Limits.Jog.Default.MaxAbsolute)
	fmt.Fprintf(w, "Bed target, at most:     %g C (default for every model)\n", view.Limits.Temperature.Default.BedMaxC)
	fmt.Fprintf(w, "Jog must be armed:       %s (an arming lasts at most %d s)\n", yesNo(view.Limits.Jog.ArmRequired), view.Limits.Jog.ArmTTLMs/1000)
	fmt.Fprintf(w, "Reservation names its controller: %s\n", yesNo(view.Limits.Claims.RequireControllerID))
	if view.Robot != nil {
		fmt.Fprintf(w, "\nRobot %d (%s):\n", view.Robot.ID, printable(view.Robot.Model))
		fmt.Fprintf(w, "  relative jog, at most: %g\n  absolute jog, at most: %g\n  bed target, at most:   %g C\n", view.Robot.Jog.MaxAmount, view.Robot.Jog.MaxAbsolute, view.Robot.Temperature.BedMaxC)
		if view.Robot.JogArmedUntil != nil {
			fmt.Fprintf(w, "  jog armed for:         %s\n", remaining(*view.Robot.JogArmedUntil))
		} else {
			fmt.Fprintln(w, "  jog armed for:         not armed")
		}
	}
	return nil
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

// ---- jog arming and jog -----------------------------------------------------

// cmdJogArm implements `hydra-cli jog-arm <robot> <on|off> [--ttl-ms N]`.
func cmdJogArm(w io.Writer, args []string) error {
	server, timeout, controllerID, rest, err := parseControlArgs(args)
	if err != nil {
		return err
	}
	ttlRaw, rest, err := takeFlag(rest, "--ttl-ms")
	if err != nil {
		return err
	}
	if len(rest) != 2 || (rest[1] != "on" && rest[1] != "off") {
		return newCliError(ExitUsageError, errors.New("usage: hydra-cli jog-arm <robotId> <on|off> [--ttl-ms N] [--controller-id ID]"))
	}
	robotID, err := parseRobotID(rest[0])
	if err != nil {
		return err
	}
	body := map[string]any{"armed": rest[1] == "on", "controllerId": controllerID}
	if ttlRaw != "" {
		ttl, convErr := strconv.Atoi(ttlRaw)
		if convErr != nil || ttl <= 0 {
			return newCliError(ExitUsageError, fmt.Errorf("--ttl-ms must be a positive whole number, got %q", ttlRaw))
		}
		body["ttlMs"] = ttl
	}
	client := http.Client{Timeout: timeout}
	resp, err := sendAuthenticated(&client, server, http.MethodPost, fmt.Sprintf("/api/robot/%d/jog-arm", robotID), body, controllerID)
	if err != nil {
		return err
	}
	if !okStatus(resp.Status) {
		return refusalError("Arming the jog", resp)
	}
	if rest[1] == "off" {
		fmt.Fprintf(w, "The jog of robot %d is disarmed.\n", robotID)
		return nil
	}
	var armed struct {
		ExpiresAt int64 `json:"expiresAt"`
	}
	_ = json.Unmarshal(resp.Body, &armed)
	fmt.Fprintf(w, "The jog of robot %d is armed for %s; it also ends with a stop or a release.\n", robotID, remaining(armed.ExpiresAt))
	return nil
}

// checkJogAgainstLimit refuses, before anything is sent, a jog above the limit
// the server holds for the model of the robot.
func checkJogAgainstLimit(amount float64, absolute bool, limits jogLimits) error {
	limit, kind := limits.MaxAmount, "relative"
	if absolute {
		limit, kind = limits.MaxAbsolute, "absolute"
	}
	if limit <= 0 || math.IsNaN(amount) || math.IsInf(amount, 0) || math.Abs(amount) > limit {
		return newCliError(ExitRefused, fmt.Errorf("jog refused: %g is above the %s limit of %g set by the server for this model; nothing was sent", amount, kind, limit))
	}
	return nil
}

// cmdJog implements `hydra-cli jog <robot> <axis> <amount> [--absolute]`.
func cmdJog(w io.Writer, args []string) error {
	server, timeout, controllerID, rest, err := parseControlArgs(args)
	if err != nil {
		return err
	}
	absolute, rest := takeSwitch(rest, "--absolute")
	if len(rest) != 3 {
		return newCliError(ExitUsageError, errors.New("usage: hydra-cli jog <robotId> <axis> <amount> [--absolute] [--controller-id ID]"))
	}
	robotID, err := parseRobotID(rest[0])
	if err != nil {
		return err
	}
	axis := strings.ToLower(rest[1])
	if len(axis) != 1 || !strings.Contains("xyzabc", axis) {
		return newCliError(ExitUsageError, fmt.Errorf("the axis must be one of x y z a b c, got %q", rest[1]))
	}
	amount, convErr := strconv.ParseFloat(rest[2], 64)
	if convErr != nil || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return newCliError(ExitUsageError, fmt.Errorf("the amount must be a finite number, got %q", rest[2]))
	}
	client := http.Client{Timeout: timeout}
	view, _, err := fetchSafety(&client, server, controllerID, &robotID)
	if err != nil {
		return err
	}
	if view.Robot == nil {
		return newCliError(ExitServerError, errors.New("the server did not report the limits of this robot; nothing was sent"))
	}
	if err := checkJogAgainstLimit(amount, absolute, view.Robot.Jog); err != nil {
		return err
	}
	params := map[string]any{"axis": axis, "amount": amount}
	if absolute {
		params["absolute"] = true
	}
	resp, err := sendAuthenticated(&client, server, http.MethodPost, fmt.Sprintf("/api/robot/%d/command", robotID),
		map[string]any{"command": "jog", "params": params, "controllerId": controllerID}, controllerID)
	if err != nil {
		return err
	}
	if !okStatus(resp.Status) {
		return refusalError("Jog", resp)
	}
	var answer struct {
		Warnings []struct {
			Field   string `json:"field"`
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"warnings"`
	}
	_ = json.Unmarshal(resp.Body, &answer)
	if len(answer.Warnings) > 0 {
		for _, warning := range answer.Warnings {
			label := warning.Field
			if warning.Code != "" {
				label = warning.Code
			}
			fmt.Fprintf(w, "Warning (%s): %s\n", printable(label), printable(warning.Message))
		}
		return newCliError(ExitRefused, errors.New("the server did not apply the jog (see the warnings above)"))
	}
	fmt.Fprintf(w, "Jog sent to robot %d: %s %g%s.\n", robotID, axis, amount, map[bool]string{true: " (absolute)", false: ""}[absolute])
	return nil
}
