// =============================================================================
// HYDRA-UMC-TOOL-CLI - Read-only views of the server: cmd/hydra-cli/observe.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0-or-later - see LICENSE
// =============================================================================
// Three read-only commands for routes of HYDRA-UMC-SERVER that the contracts of the SDK describe:
//
//	connections [clients|events]   GET /api/connections/clients and /api/connections/events
//	liveness                       GET /api/robots/liveness
//	ecosystem-health               GET /api/ecosystem/health
//
// They send nothing but the GET. Each answer is checked against the closed shape of its contract before a
// byte is printed: an answer that is not an object, that lacks a field the contract requires, that has a
// field of the wrong type or a value outside the closed lists, or whose summary does not match its list,
// is an error (exit 5) and prints nothing. What is printed is the checked content, re-encoded: a field the
// contract does not know is never passed on, and every text goes through printable.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
)

var (
	connectionKinds   = []string{"studio", "suite", "android", "ios", "dsi", "watch", "voice", "other"}
	connectionActions = []string{"first_seen", "login", "login_failed", "second_factor_failed", "logout", "session_ended", "session_expired"}
	connectionOrigins = []string{"local", "remote"}
)

func oneOf(value string, list []string) bool {
	for _, item := range list {
		if value == item {
			return true
		}
	}
	return false
}

// malformedAnswer is the error of an answer that does not have the shape of its contract.
func malformedAnswer(server, path, why string) error {
	return newCliError(ExitServerError, fmt.Errorf("unexpected response from %s%s: %s; nothing was printed", server, path, why))
}

// cleanForOutput re-reads a checked value as plain JSON data and passes every text of it through printable, so the
// object that is printed holds only what was checked and nothing a server could use to drive a terminal.
func cleanForOutput(value any) (any, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, newCliError(ExitGeneralError, err)
	}
	var generic any
	if err := json.Unmarshal(encoded, &generic); err != nil {
		return nil, newCliError(ExitGeneralError, err)
	}
	return cleanStrings(generic), nil
}

func cleanStrings(value any) any {
	switch typed := value.(type) {
	case string:
		return printable(typed)
	case []any:
		for i := range typed {
			typed[i] = cleanStrings(typed[i])
		}
		return typed
	case map[string]any:
		for key, item := range typed {
			typed[key] = cleanStrings(item)
		}
		return typed
	}
	return value
}

// readObserved does the GET of one of these commands and returns the body of a 200 answer.
func readObserved(w io.Writer, command string, args []string, build func([]string) (string, []string, error)) (server, path string, body []byte, jsonOutput bool, err error) {
	var timeout time.Duration
	var rest []string
	server, timeout, rest, err = resolveTarget(args)
	if err != nil {
		return
	}
	jsonOutput, rest = takeSwitch(rest, "--json")
	path, rest, err = build(rest)
	if err != nil {
		return
	}
	if err = rejectExtraArgs(command, rest); err != nil {
		return
	}
	client := newClient(timeout)
	var status int
	status, body, err = getAuthenticated(&client, server, path)
	if err != nil {
		return
	}
	switch status {
	case http.StatusOK:
		return
	case http.StatusNotFound:
		err = newCliError(ExitServerError, fmt.Errorf("%s does not offer %s (HTTP 404): an older server, or the feature is switched off in its configuration", server, strings.SplitN(path, "?", 2)[0]))
	case http.StatusBadRequest:
		err = newCliError(ExitRefused, fmt.Errorf("%s refused the request (HTTP 400): %s", server, printable(string(body))))
	default:
		err = newCliError(ExitServerError, fmt.Errorf("%s replied with HTTP %d: %s", server, status, printable(string(body))))
	}
	return
}

// ---- connections ------------------------------------------------------------

type connectionClient struct {
	SchemaVersion  string `json:"schema_version"`
	ID             string `json:"id"`
	Kind           string `json:"kind"`
	User           string `json:"user,omitempty"`
	AppVersion     string `json:"app_version,omitempty"`
	Origin         string `json:"origin,omitempty"`
	TwoFactor      *bool  `json:"two_factor,omitempty"`
	FirstSeenUTC   string `json:"first_seen_utc"`
	LastSeenUTC    string `json:"last_seen_utc"`
	LastFailureUTC string `json:"last_failure_utc,omitempty"`
	ActiveSessions *int   `json:"active_sessions"`
	LoginsOK       *int   `json:"logins_ok"`
	LoginsFailed   *int   `json:"logins_failed"`
	IP             string `json:"ip,omitempty"`
	Agent          string `json:"agent,omitempty"`
}

type connectionEvent struct {
	SchemaVersion string `json:"schema_version"`
	ID            *int   `json:"id"`
	AtUTC         string `json:"at_utc"`
	Action        string `json:"action"`
	ClientKind    string `json:"client_kind"`
	ClientID      string `json:"client_id"`
	User          string `json:"user,omitempty"`
	Origin        string `json:"origin,omitempty"`
	TwoFactor     *bool  `json:"two_factor,omitempty"`
	AppVersion    string `json:"app_version,omitempty"`
	IP            string `json:"ip,omitempty"`
	Agent         string `json:"agent,omitempty"`
}

func nonNegative(value *int) bool { return value != nil && *value >= 0 }

func checkOrigin(origin string) bool { return origin == "" || oneOf(origin, connectionOrigins) }

func validateClients(list []connectionClient) error {
	for i, c := range list {
		switch {
		case c.SchemaVersion != "1.0":
			return fmt.Errorf("client %d has schema_version %q, not 1.0", i, c.SchemaVersion)
		case c.ID == "":
			return fmt.Errorf("client %d has no id", i)
		case !oneOf(c.Kind, connectionKinds):
			return fmt.Errorf("client %d has the kind %q, which the contract does not list", i, c.Kind)
		case !checkOrigin(c.Origin):
			return fmt.Errorf("client %d has the origin %q", i, c.Origin)
		case c.FirstSeenUTC == "" || c.LastSeenUTC == "":
			return fmt.Errorf("client %d lacks first_seen_utc or last_seen_utc", i)
		case !nonNegative(c.ActiveSessions) || !nonNegative(c.LoginsOK) || !nonNegative(c.LoginsFailed):
			return fmt.Errorf("client %d lacks a count (active_sessions, logins_ok, logins_failed) or it is negative", i)
		}
	}
	return nil
}

func validateEvents(list []connectionEvent) error {
	for i, e := range list {
		switch {
		case e.SchemaVersion != "1.0":
			return fmt.Errorf("event %d has schema_version %q, not 1.0", i, e.SchemaVersion)
		case !nonNegative(e.ID):
			return fmt.Errorf("event %d has no id", i)
		case e.AtUTC == "":
			return fmt.Errorf("event %d has no at_utc", i)
		case !oneOf(e.Action, connectionActions):
			return fmt.Errorf("event %d has the action %q, which the contract does not list", i, e.Action)
		case !oneOf(e.ClientKind, connectionKinds):
			return fmt.Errorf("event %d has the client_kind %q, which the contract does not list", i, e.ClientKind)
		case e.ClientID == "":
			return fmt.Errorf("event %d has no client_id", i)
		case !checkOrigin(e.Origin):
			return fmt.Errorf("event %d has the origin %q", i, e.Origin)
		}
	}
	return nil
}

// wholeNumber checks a --limit or --before value the way the server does (1 to 9 digits).
func wholeNumber(flag, raw string) error {
	if len(raw) < 1 || len(raw) > 9 || strings.Trim(raw, "0123456789") != "" {
		return newCliError(ExitUsageError, fmt.Errorf("%s must be a whole number of 1 to 9 digits, got %q", flag, raw))
	}
	return nil
}

// cmdConnections implements `hydra-cli connections [clients|events] [filters] [--json]`.
func cmdConnections(w io.Writer, args []string) error {
	sub := "clients"
	var kind, user, action, limit, before string
	var active bool
	server, path, body, jsonOutput, err := readObserved(w, "connections", args, func(rest []string) (string, []string, error) {
		if len(rest) > 0 && (rest[0] == "clients" || rest[0] == "events") {
			sub, rest = rest[0], rest[1:]
		}
		var err error
		if kind, rest, err = takeFlag(rest, "--kind"); err != nil {
			return "", nil, err
		}
		if user, rest, err = takeFlag(rest, "--user"); err != nil {
			return "", nil, err
		}
		if action, rest, err = takeFlag(rest, "--action"); err != nil {
			return "", nil, err
		}
		if limit, rest, err = takeFlag(rest, "--limit"); err != nil {
			return "", nil, err
		}
		if before, rest, err = takeFlag(rest, "--before"); err != nil {
			return "", nil, err
		}
		active, rest = takeSwitch(rest, "--active")
		if kind != "" && !oneOf(kind, connectionKinds) {
			return "", nil, newCliError(ExitUsageError, fmt.Errorf("--kind must be one of %s, got %q", strings.Join(connectionKinds, ", "), kind))
		}
		query := url.Values{}
		if kind != "" {
			query.Set("kind", kind)
		}
		if user != "" {
			query.Set("user", user)
		}
		switch sub {
		case "clients":
			if action != "" || limit != "" || before != "" {
				return "", nil, newCliError(ExitUsageError, errors.New("--action, --limit and --before belong to `connections events`"))
			}
			if active {
				query.Set("active", "1")
			}
		case "events":
			if active {
				return "", nil, newCliError(ExitUsageError, errors.New("--active belongs to `connections clients`"))
			}
			if action != "" {
				if !oneOf(action, connectionActions) {
					return "", nil, newCliError(ExitUsageError, fmt.Errorf("--action must be one of %s, got %q", strings.Join(connectionActions, ", "), action))
				}
				query.Set("action", action)
			}
			if limit != "" {
				if err := wholeNumber("--limit", limit); err != nil {
					return "", nil, err
				}
				query.Set("limit", limit)
			}
			if before != "" {
				if err := wholeNumber("--before", before); err != nil {
					return "", nil, err
				}
				query.Set("before", before)
			}
		}
		path := "/api/connections/" + sub
		if encoded := query.Encode(); encoded != "" {
			path += "?" + encoded
		}
		return path, rest, nil
	})
	if err != nil {
		return err
	}
	apiPath := strings.SplitN(path, "?", 2)[0]
	if sub == "events" {
		var answer struct {
			Events []connectionEvent `json:"events"`
		}
		if err := json.Unmarshal(body, &answer); err != nil {
			return malformedAnswer(server, apiPath, err.Error())
		}
		if answer.Events == nil {
			return malformedAnswer(server, apiPath, "there is no events list")
		}
		if err := validateEvents(answer.Events); err != nil {
			return malformedAnswer(server, apiPath, err.Error())
		}
		clean, err := cleanForOutput(answer.Events)
		if err != nil {
			return err
		}
		if jsonOutput {
			return writeJSONResult(w, "connections events", true, map[string]any{"events": clean})
		}
		printConnectionEvents(w, answer.Events)
		return nil
	}
	var answer struct {
		Clients []connectionClient `json:"clients"`
	}
	if err := json.Unmarshal(body, &answer); err != nil {
		return malformedAnswer(server, apiPath, err.Error())
	}
	if answer.Clients == nil {
		return malformedAnswer(server, apiPath, "there is no clients list")
	}
	if err := validateClients(answer.Clients); err != nil {
		return malformedAnswer(server, apiPath, err.Error())
	}
	clean, err := cleanForOutput(answer.Clients)
	if err != nil {
		return err
	}
	if jsonOutput {
		return writeJSONResult(w, "connections clients", true, map[string]any{"clients": clean})
	}
	printConnectionClients(w, answer.Clients)
	return nil
}

func dashIfEmpty(text string) string {
	if text == "" {
		return "-"
	}
	return printable(text)
}

func printConnectionClients(w io.Writer, list []connectionClient) {
	if len(list) == 0 {
		fmt.Fprintln(w, "No client has connected.")
		return
	}
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "KIND\tUSER\tVERSION\tORIGIN\tSESSIONS\tLOGINS OK\tFAILED\tLAST SEEN\tID")
	for _, c := range list {
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%d\t%d\t%d\t%s\t%s\n", printable(c.Kind), dashIfEmpty(c.User), dashIfEmpty(c.AppVersion), dashIfEmpty(c.Origin),
			*c.ActiveSessions, *c.LoginsOK, *c.LoginsFailed, printable(c.LastSeenUTC), printable(c.ID))
	}
	table.Flush()
}

func printConnectionEvents(w io.Writer, list []connectionEvent) {
	if len(list) == 0 {
		fmt.Fprintln(w, "No connection event.")
		return
	}
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "N\tAT\tACTION\tKIND\tUSER\tORIGIN\tCLIENT")
	for _, e := range list {
		fmt.Fprintf(table, "%d\t%s\t%s\t%s\t%s\t%s\t%s\n", *e.ID, printable(e.AtUTC), printable(e.Action), printable(e.ClientKind), dashIfEmpty(e.User), dashIfEmpty(e.Origin), printable(e.ClientID))
	}
	table.Flush()
}

// ---- liveness ---------------------------------------------------------------

type robotLiveness struct {
	ControllerID  string   `json:"controller_id"`
	RobotID       *int     `json:"robot_id"`
	State         string   `json:"state"`
	LastUpdateUTC string   `json:"last_update_utc,omitempty"`
	AgeS          *float64 `json:"age_s,omitempty"`
	Source        string   `json:"source,omitempty"`
}

type livenessReport struct {
	SchemaVersion string          `json:"schema_version"`
	StaleAfterS   *int            `json:"stale_after_s"`
	Robots        []robotLiveness `json:"robots"`
}

func validateLiveness(report livenessReport) error {
	if report.SchemaVersion != "1.0" {
		return fmt.Errorf("schema_version is %q, not 1.0", report.SchemaVersion)
	}
	if report.StaleAfterS == nil || *report.StaleAfterS < 1 {
		return errors.New("stale_after_s is missing or below 1")
	}
	if report.Robots == nil {
		return errors.New("there is no robots list")
	}
	for i, r := range report.Robots {
		switch {
		case r.ControllerID == "":
			return fmt.Errorf("robot %d has no controller_id", i)
		case !nonNegative(r.RobotID):
			return fmt.Errorf("robot %d has no robot_id", i)
		case !oneOf(r.State, []string{"live", "stale", "unknown"}):
			return fmt.Errorf("robot %d has the state %q", i, r.State)
		case r.Source != "" && !oneOf(r.Source, []string{"state", "heartbeat"}):
			return fmt.Errorf("robot %d has the source %q", i, r.Source)
		case r.AgeS != nil && *r.AgeS < 0:
			return fmt.Errorf("robot %d has a negative age_s", i)
		case r.State == "unknown" && (r.LastUpdateUTC != "" || r.AgeS != nil):
			return fmt.Errorf("robot %d is unknown but reports news", i)
		case r.State != "unknown" && (r.LastUpdateUTC == "" || r.AgeS == nil):
			return fmt.Errorf("robot %d is %s but reports no last_update_utc or age_s", i, r.State)
		}
	}
	return nil
}

// cmdLiveness implements `hydra-cli liveness [--json]`.
func cmdLiveness(w io.Writer, args []string) error {
	server, path, body, jsonOutput, err := readObserved(w, "liveness", args, func(rest []string) (string, []string, error) {
		return "/api/robots/liveness", rest, nil
	})
	if err != nil {
		return err
	}
	var report livenessReport
	if err := json.Unmarshal(body, &report); err != nil {
		return malformedAnswer(server, path, err.Error())
	}
	if err := validateLiveness(report); err != nil {
		return malformedAnswer(server, path, err.Error())
	}
	if jsonOutput {
		clean, err := cleanForOutput(report)
		if err != nil {
			return err
		}
		fields, _ := clean.(map[string]any)
		delete(fields, "schema_version")
		return writeJSONResult(w, "liveness", true, fields)
	}
	if len(report.Robots) == 0 {
		fmt.Fprintf(w, "The server lists no robot (a robot with no news for %d s is stale).\n", *report.StaleAfterS)
		return nil
	}
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "CONTROLLER\tROBOT\tSTATE\tAGE (s)\tSOURCE\tLAST NEWS")
	for _, r := range report.Robots {
		age := "-"
		if r.AgeS != nil {
			age = strconv.FormatFloat(*r.AgeS, 'f', -1, 64)
		}
		fmt.Fprintf(table, "%s\t%d\t%s\t%s\t%s\t%s\n", printable(r.ControllerID), *r.RobotID, printable(r.State), age, dashIfEmpty(r.Source), dashIfEmpty(r.LastUpdateUTC))
	}
	table.Flush()
	fmt.Fprintf(w, "A robot with no news for %d s is stale; one that never reported since the server started is unknown.\n", *report.StaleAfterS)
	return nil
}

// ---- ecosystem-health -------------------------------------------------------

type serviceHealth struct {
	Name            string   `json:"name"`
	Family          *string  `json:"family"`
	Role            *string  `json:"role"`
	State           string   `json:"state"`
	Reason          string   `json:"reason"`
	VersionExpected *string  `json:"version_expected"`
	VersionReported *string  `json:"version_reported"`
	Drift           *bool    `json:"drift"`
	LastSeenUTC     *string  `json:"last_seen_utc"`
	LatencyMs       *float64 `json:"latency_ms"`
	Port            *int     `json:"port,omitempty"`
	HealthPath      *string  `json:"health_path,omitempty"`
	SystemdUnit     *string  `json:"systemd_unit,omitempty"`
	ActiveState     *string  `json:"active_state,omitempty"`
}

type certificateHealth struct {
	State    string   `json:"state"`
	NotAfter *string  `json:"not_after_utc"`
	DaysLeft *float64 `json:"days_left"`
	WarnDays *int     `json:"warn_days"`
}

type healthWarning struct {
	Code    string `json:"code"`
	Service string `json:"service,omitempty"`
}

type healthSummary struct {
	Total    *int `json:"total"`
	OK       *int `json:"ok"`
	Degraded *int `json:"degraded"`
	Down     *int `json:"down"`
	Unknown  *int `json:"unknown"`
	Drift    *int `json:"drift"`
}

type ecosystemReport struct {
	SchemaVersion  string             `json:"schema_version"`
	GeneratedAtUTC string             `json:"generated_at_utc"`
	Available      *bool              `json:"available"`
	Summary        *healthSummary     `json:"summary"`
	Services       []serviceHealth    `json:"services"`
	Certificate    *certificateHealth `json:"certificate"`
	Warnings       []healthWarning    `json:"warnings"`
}

var (
	serviceStates  = []string{"ok", "degraded", "down", "unknown"}
	serviceReasons = []string{"health_ok", "health_degraded", "health_failed", "http_error", "unreachable", "reachable_unit_not_active", "unreachable_unit_active",
		"unit_active", "unit_starting", "unit_failed", "unit_inactive", "no_signal", "probe_error"}
	certificateStates = []string{"ok", "expiring", "expired", "unreadable"}
	warningCodes      = []string{"certificate_expiring", "certificate_expired", "certificate_unreadable", "version_drift", "service_down", "service_degraded"}
)

func validateEcosystem(report ecosystemReport) error {
	if report.SchemaVersion != "1.0" {
		return fmt.Errorf("schema_version is %q, not 1.0", report.SchemaVersion)
	}
	if report.GeneratedAtUTC == "" {
		return errors.New("generated_at_utc is missing")
	}
	if report.Available == nil {
		return errors.New("available is missing")
	}
	s := report.Summary
	if s == nil || !nonNegative(s.Total) || !nonNegative(s.OK) || !nonNegative(s.Degraded) || !nonNegative(s.Down) || !nonNegative(s.Unknown) || !nonNegative(s.Drift) {
		return errors.New("the summary is missing or has a missing or negative count")
	}
	if report.Services == nil || report.Warnings == nil {
		return errors.New("the services list or the warnings list is missing")
	}
	tally := map[string]int{}
	drift := 0
	for i, svc := range report.Services {
		switch {
		case svc.Name == "":
			return fmt.Errorf("service %d has no name", i)
		case !oneOf(svc.State, serviceStates):
			return fmt.Errorf("service %d has the state %q", i, svc.State)
		case !oneOf(svc.Reason, serviceReasons):
			return fmt.Errorf("service %d has the reason %q", i, svc.Reason)
		case svc.Drift == nil:
			return fmt.Errorf("service %d lacks drift", i)
		case svc.LatencyMs != nil && *svc.LatencyMs < 0:
			return fmt.Errorf("service %d has a negative latency", i)
		}
		tally[svc.State]++
		if *svc.Drift {
			drift++
		}
	}
	if *s.Total != len(report.Services) || *s.OK != tally["ok"] || *s.Degraded != tally["degraded"] || *s.Down != tally["down"] || *s.Unknown != tally["unknown"] || *s.Drift != drift {
		return errors.New("the summary does not match the list of services")
	}
	if c := report.Certificate; c != nil {
		if !oneOf(c.State, certificateStates) || c.WarnDays == nil || *c.WarnDays < 0 {
			return errors.New("the certificate has a state outside the contract or no warn_days")
		}
	}
	for i, warning := range report.Warnings {
		if !oneOf(warning.Code, warningCodes) {
			return fmt.Errorf("warning %d has the code %q", i, warning.Code)
		}
	}
	return nil
}

func textOrDash(text *string) string {
	if text == nil || *text == "" {
		return "-"
	}
	return printable(*text)
}

// cmdEcosystemHealth implements `hydra-cli ecosystem-health [--json]`.
func cmdEcosystemHealth(w io.Writer, args []string) error {
	server, path, body, jsonOutput, err := readObserved(w, "ecosystem-health", args, func(rest []string) (string, []string, error) {
		return "/api/ecosystem/health", rest, nil
	})
	if err != nil {
		return err
	}
	var report ecosystemReport
	if err := json.Unmarshal(body, &report); err != nil {
		return malformedAnswer(server, path, err.Error())
	}
	if err := validateEcosystem(report); err != nil {
		return malformedAnswer(server, path, err.Error())
	}
	if jsonOutput {
		clean, err := cleanForOutput(report)
		if err != nil {
			return err
		}
		fields, _ := clean.(map[string]any)
		delete(fields, "schema_version")
		return writeJSONResult(w, "ecosystem-health", true, fields)
	}
	s := report.Summary
	fmt.Fprintf(w, "Services: %d (ok %d, degraded %d, down %d, unknown %d); version drift: %d. Generated %s.\n", *s.Total, *s.OK, *s.Degraded, *s.Down, *s.Unknown, *s.Drift, printable(report.GeneratedAtUTC))
	if !*report.Available {
		fmt.Fprintln(w, "The server could not scan the repositories next to it: there is no service list.")
	}
	if len(report.Services) > 0 {
		table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintln(table, "SERVICE\tSTATE\tREASON\tEXPECTED\tREPORTED\tDRIFT\tLATENCY (ms)")
		for _, svc := range report.Services {
			latency := "-"
			if svc.LatencyMs != nil {
				latency = strconv.FormatFloat(*svc.LatencyMs, 'f', -1, 64)
			}
			fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", printable(svc.Name), printable(svc.State), printable(svc.Reason), textOrDash(svc.VersionExpected), textOrDash(svc.VersionReported), yesNo(*svc.Drift), latency)
		}
		table.Flush()
	}
	if c := report.Certificate; c != nil {
		days := "-"
		if c.DaysLeft != nil {
			days = strconv.FormatFloat(*c.DaysLeft, 'f', -1, 64)
		}
		fmt.Fprintf(w, "Certificate: %s (valid until %s, %s days left, warns %d days before)\n", printable(c.State), textOrDash(c.NotAfter), days, *c.WarnDays)
	}
	for _, warning := range report.Warnings {
		if warning.Service != "" {
			fmt.Fprintf(w, "Warning: %s (%s)\n", printable(warning.Code), printable(warning.Service))
		} else {
			fmt.Fprintf(w, "Warning: %s\n", printable(warning.Code))
		}
	}
	return nil
}
