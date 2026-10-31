// =============================================================================
// HYDRA-UMC-TOOL-CLI - Robot revision, table move and audit record: cmd/hydra-cli/record.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0 - see LICENSE
// =============================================================================
// The second contract of HYDRA-UMC-SERVER:
//   - `robot` reads one robot with its revision, which `--if-match` of claim,
//     release, jog and table sends back so the server refuses a write to a
//     robot that changed in between;
//   - `table` moves the XY table of a robot in one atomic request;
//   - `audit` reads the append-only audit record (administrators only) and checks the chain of SHA-256 hashes
//     of the lines it was given, optionally asking the server to check the whole record.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
)

// ---- robot --------------------------------------------------------------------

// cmdRobot implements `hydra-cli robot <robotId> [--json]`.
func cmdRobot(w io.Writer, args []string) error {
	server, timeout, controllerID, rest, err := parseControlArgs(args)
	if err != nil {
		return err
	}
	jsonOutput, rest := takeSwitch(rest, "--json")
	if len(rest) != 1 {
		return newCliError(ExitUsageError, errors.New("usage: hydra-cli robot <robotId> [--json] [--controller-id ID]"))
	}
	robotID, err := parseRobotID(rest[0])
	if err != nil {
		return err
	}
	client := newClient(timeout)
	resp, err := sendAuthenticated(&client, server, http.MethodGet, fmt.Sprintf("/api/robot/%d", robotID), nil, controllerID)
	if err != nil {
		return err
	}
	if !okStatus(resp.Status) {
		return refusalError("Reading the robot", resp)
	}
	if jsonOutput {
		var indented bytes.Buffer
		if json.Indent(&indented, resp.Body, "", "  ") != nil {
			indented.Write(resp.Body)
		}
		_, err := fmt.Fprintln(w, indented.String())
		return err
	}
	var view struct {
		Robot struct {
			ID    json.Number `json:"id"`
			Name  string      `json:"name"`
			Model string      `json:"model"`
			Role  string      `json:"role"`
		} `json:"robot"`
		Revision string `json:"revision"`
		CellID   string `json:"cellId"`
	}
	if err := json.Unmarshal(resp.Body, &view); err != nil {
		return newCliError(ExitServerError, fmt.Errorf("unexpected response from %s: %w", server, err))
	}
	fmt.Fprintf(w, "Robot %d: %s (%s)\n", robotID, printable(view.Robot.Name), printable(view.Robot.Model))
	if view.Robot.Role != "" {
		fmt.Fprintf(w, "Role:     %s\n", printable(view.Robot.Role))
	}
	if view.CellID != "" {
		fmt.Fprintf(w, "Cell:     %s\n", printable(view.CellID))
	}
	if view.Revision == "" || !revisionPattern.MatchString(view.Revision) {
		fmt.Fprintln(w, "Revision: not reported (an older server)")
		return nil
	}
	fmt.Fprintf(w, "Revision: %s\n", view.Revision)
	return nil
}

// ---- table --------------------------------------------------------------------

// checkTableAgainstLimit refuses, before anything is sent, a table move above the limits the server holds for
// the model of the robot: every number of a relative move within the largest jog, every number of an absolute
// move within the largest absolute position. The server also checks where a relative move ends.
func checkTableAgainstLimit(values []float64, relative bool, limits jogLimits) error {
	limit, kind := limits.MaxAbsolute, "absolute"
	if relative {
		limit, kind = limits.MaxAmount, "relative"
	}
	if limit <= 0 {
		return newCliError(ExitRefused, fmt.Errorf("table move refused: the server reports no %s limit for this model; nothing was sent", kind))
	}
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > limit {
			return newCliError(ExitRefused, fmt.Errorf("table move refused: %g is above the %s limit of %g set by the server for this model; nothing was sent", value, kind, limit))
		}
	}
	return nil
}

// cmdTable implements `hydra-cli table <robot> <x> <y> [--rz N] [--relative] [--if-match REVISION|current]`.
func cmdTable(w io.Writer, args []string) error {
	server, timeout, controllerID, rest, err := parseControlArgs(args)
	if err != nil {
		return err
	}
	relative, rest := takeSwitch(rest, "--relative")
	jsonOutput, rest := takeSwitch(rest, "--json")
	rzRaw, rest, err := takeFlag(rest, "--rz")
	if err != nil {
		return err
	}
	ifMatch, rest, err := takeIfMatch(rest)
	if err != nil {
		return err
	}
	if len(rest) != 3 {
		return newCliError(ExitUsageError, errors.New("usage: hydra-cli table <robotId> <x> <y> [--rz N] [--relative] [--if-match REVISION|current] [--controller-id ID]"))
	}
	robotID, err := parseRobotID(rest[0])
	if err != nil {
		return err
	}
	x, xErr := strconv.ParseFloat(rest[1], 64)
	y, yErr := strconv.ParseFloat(rest[2], 64)
	if xErr != nil || yErr != nil || math.IsNaN(x) || math.IsInf(x, 0) || math.IsNaN(y) || math.IsInf(y, 0) {
		return newCliError(ExitUsageError, fmt.Errorf("x and y must be finite numbers, got %q and %q", rest[1], rest[2]))
	}
	values := []float64{x, y}
	mode := "absolute"
	if relative {
		mode = "relative"
	}
	params := map[string]any{"x": x, "y": y, "mode": mode}
	if rzRaw != "" {
		rz, convErr := strconv.ParseFloat(rzRaw, 64)
		if convErr != nil || math.IsNaN(rz) || math.IsInf(rz, 0) {
			return newCliError(ExitUsageError, fmt.Errorf("--rz must be a finite number, got %q", rzRaw))
		}
		params["rz"] = rz
		values = append(values, rz)
	}
	client := newClient(timeout)
	view, _, err := fetchSafety(&client, server, controllerID, &robotID)
	if err != nil {
		return err
	}
	if view.Robot == nil {
		return newCliError(ExitServerError, errors.New("the server did not report the limits of this robot; nothing was sent"))
	}
	if err := checkTableAgainstLimit(values, relative, view.Robot.Jog); err != nil {
		return err
	}
	ifMatch, err = resolveIfMatch(&client, server, controllerID, robotID, ifMatch)
	if err != nil {
		return err
	}
	resp, err := sendConditional(&client, server, http.MethodPost, fmt.Sprintf("/api/robot/%d/command", robotID),
		map[string]any{"command": "table", "params": params, "controllerId": controllerID}, controllerID, ifMatch)
	if err != nil {
		return err
	}
	if resp.Status == http.StatusBadRequest && resp.detail() == "unknown command" {
		return newCliError(ExitRefused, errors.New("this server does not have the table command (an older version); use `hydra-cli jog <robot> x|y <amount> --absolute` twice instead"))
	}
	if !okStatus(resp.Status) {
		return refusalError("Table move", resp)
	}
	var answer struct {
		Applied struct {
			X  float64  `json:"x"`
			Y  float64  `json:"y"`
			Rz *float64 `json:"rz"`
		} `json:"applied"`
	}
	_ = json.Unmarshal(resp.Body, &answer)
	if jsonOutput {
		return writeJSONResult(w, "table", true, map[string]any{"robot": robotID, "x": answer.Applied.X, "y": answer.Applied.Y, "rz": answer.Applied.Rz, "revision": revisionOrNil(resp)})
	}
	if answer.Applied.Rz != nil {
		fmt.Fprintf(w, "Table of robot %d is now at x %g, y %g, rz %g.\n", robotID, answer.Applied.X, answer.Applied.Y, *answer.Applied.Rz)
	} else {
		fmt.Fprintf(w, "Table of robot %d is now at x %g, y %g.\n", robotID, answer.Applied.X, answer.Applied.Y)
	}
	printRevision(w, resp)
	return nil
}

// ---- audit --------------------------------------------------------------------

var auditHashSuffix = regexp.MustCompile(`,"hash":"([0-9a-f]{64})"\}$`)

type auditLine struct {
	Seq          int64          `json:"seq"`
	TS           string         `json:"ts"`
	Type         string         `json:"type"`
	Outcome      string         `json:"outcome"`
	Username     string         `json:"username"`
	ControllerID string         `json:"controllerId"`
	IP           string         `json:"ip"`
	RobotID      *int           `json:"robotId"`
	Detail       map[string]any `json:"detail"`
	Prev         string         `json:"prev"`
	Hash         string         `json:"hash"`
	// hashed is the SHA-256 of the line as the server wrote it, without its hash; "" when the line could not be re-read byte for byte.
	hashed string
}

type auditRead struct {
	Enabled      bool              `json:"enabled"`
	LockedOff    bool              `json:"lockedOff"`
	Entries      []json.RawMessage `json:"entries"`
	Verification *struct {
		OK          bool   `json:"ok"`
		Entries     int    `json:"entries"`
		BrokenAtSeq *int64 `json:"brokenAtSeq"`
		Reason      string `json:"reason"`
	} `json:"verification"`
}

// parseAuditLines reads the lines of the record and the hash each one should have: the server hashes the line as
// compact JSON without its `hash` (which is its last key), so the bytes it sent minus that suffix are what was hashed.
func parseAuditLines(raw []json.RawMessage) []auditLine {
	lines := make([]auditLine, 0, len(raw))
	for _, item := range raw {
		var line auditLine
		if json.Unmarshal(item, &line) != nil || line.Hash == "" || line.Prev == "" {
			continue
		}
		text := strings.TrimSpace(string(item))
		if match := auditHashSuffix.FindStringSubmatchIndex(text); match != nil && !strings.Contains(text, "\n") {
			sum := sha256.Sum256([]byte(text[:match[0]] + "}"))
			line.hashed = hex.EncodeToString(sum[:])
		}
		lines = append(lines, line)
	}
	return lines
}

// chainCheck is the outcome of checking the lines this program was given.
type chainCheck struct {
	OK          bool
	Checked     int
	Links       int
	Gaps        int
	Unreadable  int
	BrokenAtSeq int64
	Reason      string // "changed" | "link"
}

// checkAuditChain checks the hash of every line and the link between lines that are neighbours (sequence numbers one apart).
func checkAuditChain(lines []auditLine) chainCheck {
	result := chainCheck{OK: true}
	for i, line := range lines {
		if line.hashed == "" {
			result.Unreadable++
		} else if line.hashed != line.Hash {
			return chainCheck{Checked: i, Links: result.Links, Gaps: result.Gaps, BrokenAtSeq: line.Seq, Reason: "changed"}
		}
		if i == 0 {
			continue
		}
		before := lines[i-1]
		if line.Seq == before.Seq+1 {
			result.Links++
			if line.Prev != before.Hash {
				return chainCheck{Checked: i + 1, Links: result.Links, Gaps: result.Gaps, BrokenAtSeq: line.Seq, Reason: "link"}
			}
		} else {
			result.Gaps++
		}
	}
	result.Checked = len(lines)
	return result
}

func auditQuery(limit int, kind string, robotID *int, verify bool, format string) string {
	values := url.Values{}
	if limit > 0 {
		if limit > 1000 {
			limit = 1000
		}
		values.Set("limit", strconv.Itoa(limit))
	}
	if kind != "" {
		values.Set("type", kind)
	}
	if robotID != nil {
		values.Set("robotId", strconv.Itoa(*robotID))
	}
	if verify {
		values.Set("verify", "1")
	}
	if format != "" {
		values.Set("format", format)
	}
	if len(values) == 0 {
		return ""
	}
	return "?" + values.Encode()
}

// cmdAudit implements `hydra-cli audit [--limit N] [--type T] [--robot ID] [--verify] [--export FILE] [--json]`.
func cmdAudit(w io.Writer, args []string) error {
	server, timeout, controllerID, rest, err := parseControlArgs(args)
	if err != nil {
		return err
	}
	jsonOutput, rest := takeSwitch(rest, "--json")
	verify, rest := takeSwitch(rest, "--verify")
	limitRaw, rest, err := takeFlag(rest, "--limit")
	if err != nil {
		return err
	}
	kind, rest, err := takeFlag(rest, "--type")
	if err != nil {
		return err
	}
	robotRaw, rest, err := takeFlag(rest, "--robot")
	if err != nil {
		return err
	}
	exportPath, rest, err := takeFlag(rest, "--export")
	if err != nil {
		return err
	}
	if err := rejectExtraArgs("audit", rest); err != nil {
		return err
	}
	limit := 100
	if limitRaw != "" {
		n, convErr := strconv.Atoi(limitRaw)
		if convErr != nil || n < 1 || n > 1000 {
			return newCliError(ExitUsageError, fmt.Errorf("--limit must be a whole number from 1 to 1000, got %q", limitRaw))
		}
		limit = n
	}
	var robotID *int
	if robotRaw != "" {
		id, idErr := parseRobotID(robotRaw)
		if idErr != nil {
			return idErr
		}
		robotID = &id
	}
	client := newClient(timeout)
	if exportPath != "" {
		resp, err := sendAuthenticated(&client, server, http.MethodGet, "/api/audit"+auditQuery(limit, kind, robotID, false, "jsonl"), nil, controllerID)
		if err != nil {
			return err
		}
		if !okStatus(resp.Status) {
			return auditRefusal(resp)
		}
		if err := os.WriteFile(exportPath, resp.Body, 0o600); err != nil {
			return newCliError(ExitGeneralError, fmt.Errorf("could not write %s: %w", exportPath, err))
		}
		fmt.Fprintf(w, "Saved %d bytes of the audit record to %s.\n", len(resp.Body), exportPath)
		return nil
	}
	resp, err := sendAuthenticated(&client, server, http.MethodGet, "/api/audit"+auditQuery(limit, kind, robotID, verify, ""), nil, controllerID)
	if err != nil {
		return err
	}
	if !okStatus(resp.Status) {
		return auditRefusal(resp)
	}
	var read auditRead
	if err := json.Unmarshal(resp.Body, &read); err != nil {
		return newCliError(ExitServerError, fmt.Errorf("unexpected response from %s: %w", server, err))
	}
	lines := parseAuditLines(read.Entries)
	local := checkAuditChain(lines)
	if jsonOutput {
		out := map[string]any{"enabled": read.Enabled, "lockedOff": read.LockedOff, "entries": read.Entries, "chain": map[string]any{
			"ok": local.OK, "checked": local.Checked, "links": local.Links, "gaps": local.Gaps, "unreadable": local.Unreadable,
		}}
		if !local.OK {
			out["chain"].(map[string]any)["brokenAtSeq"] = local.BrokenAtSeq
			out["chain"].(map[string]any)["reason"] = local.Reason
		}
		if read.Verification != nil {
			out["verification"] = read.Verification
		}
		encoded, _ := json.MarshalIndent(out, "", "  ")
		fmt.Fprintln(w, string(encoded))
	} else {
		if !read.Enabled {
			if read.LockedOff {
				fmt.Fprintln(w, "The audit record is locked off by the environment of the server: nothing new is written.")
			} else {
				fmt.Fprintln(w, "The audit record is switched off in the safety limits: nothing new is written.")
			}
		}
		if len(lines) == 0 {
			fmt.Fprintln(w, "No lines for this filter.")
		} else {
			table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
			fmt.Fprintln(table, "SEQ\tTIME\tTYPE\tOUTCOME\tACCOUNT\tCONTROLLER\tADDRESS\tROBOT\tDETAIL")
			for i := len(lines) - 1; i >= 0; i-- {
				line := lines[i]
				robot := "-"
				if line.RobotID != nil {
					robot = strconv.Itoa(*line.RobotID)
				}
				fmt.Fprintf(table, "%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", line.Seq, auditTime(line.TS), printable(line.Type), printable(line.Outcome),
					dash(printable(line.Username)), dash(printable(line.ControllerID)), dash(printable(line.IP)), robot, auditDetail(line.Detail))
			}
			if err := table.Flush(); err != nil {
				return err
			}
		}
		if read.Verification != nil {
			if read.Verification.OK {
				fmt.Fprintf(w, "Server: the whole chain is intact (%d lines).\n", read.Verification.Entries)
			} else {
				seq := "?"
				if read.Verification.BrokenAtSeq != nil {
					seq = strconv.FormatInt(*read.Verification.BrokenAtSeq, 10)
				}
				fmt.Fprintf(w, "Server: the chain is BROKEN at line %s (%s).\n", seq, printable(read.Verification.Reason))
			}
		}
		if local.OK {
			fmt.Fprintf(w, "This program: %d lines read, every hash matches, %d links between neighbours checked, %d gaps not comparable", local.Checked, local.Links, local.Gaps)
			if local.Unreadable > 0 {
				fmt.Fprintf(w, ", %d lines could not be re-read byte for byte", local.Unreadable)
			}
			fmt.Fprintln(w, ".")
		} else if local.Reason == "link" {
			fmt.Fprintf(w, "This program: line %d does not name the line before it - a line is missing or was replaced.\n", local.BrokenAtSeq)
		} else {
			fmt.Fprintf(w, "This program: line %d does not match its own hash - it was changed.\n", local.BrokenAtSeq)
		}
	}
	if !local.OK || (read.Verification != nil && !read.Verification.OK) {
		return newCliError(ExitRefused, errors.New("the audit chain is broken"))
	}
	return nil
}

// auditRefusal says why the record could not be read.
func auditRefusal(r controlResponse) error {
	switch {
	case r.Status == http.StatusForbidden:
		return newCliError(ExitAuthError, errors.New("reading the audit record refused: this account is not an administrator"))
	case r.Status == http.StatusNotFound:
		return newCliError(ExitServerError, errors.New("this server has no audit record (an older version, or it is not available)"))
	}
	return refusalError("Reading the audit record", r)
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func auditTime(ts string) string {
	if parsed, err := time.Parse(time.RFC3339Nano, ts); err == nil {
		return parsed.UTC().Format("2006-01-02 15:04:05")
	}
	return printable(ts)
}

// auditDetail is a short key=value line of the detail of a line, in a stable order.
func auditDetail(detail map[string]any) string {
	if len(detail) == 0 {
		return ""
	}
	keys := make([]string, 0, len(detail))
	for key := range detail {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", key, detail[key]))
	}
	joined := printable(strings.Join(parts, " "))
	if len(joined) > 120 {
		return joined[:117] + "..."
	}
	return joined
}
