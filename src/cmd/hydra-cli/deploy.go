// =============================================================================
// HYDRA-UMC-TOOL-CLI - Deploy through the updater: cmd/hydra-cli/deploy.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0-or-later - see LICENSE
// =============================================================================
// `deploy` puts what the workspace has on the device by the way the project already has: the updater (HYDRA-UMC-UPDATER) builds the plan of
// what the device would need (`deploy-plan`, read only) and, only when told, applies it through the installer of the device, with its health
// check and its way back. The updater is the one that checks the signature of each checkout, honours the `os_deploy` switch and writes the
// evidence; this command does not weaken any of that and does not reach the device by itself.
//
// Where the updater runs is the operator's choice, as a command: `--updater "hydra-umc-updater"` (the default, or $HYDRA_CLI_UPDATER) on the
// machine that holds the device's workspace, or for a device reached over SSH `--updater "ssh user@device hydra-umc-updater"`. The command is
// split like a shell line (spaces and quotes) and started WITHOUT a shell. Without --apply and --yes nothing is changed anywhere.
//
// Evidence: tested against a stand-in updater (a process of the test that answers like the real one); never run against a real device.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"
	"time"
)

const (
	updaterEnv = "HYDRA_CLI_UPDATER"
	// defaultUpdater is the command of the updater when nothing else is said.
	defaultUpdater = "hydra-umc-updater"
	// maxUpdaterOutput bounds what is read from the updater.
	maxUpdaterOutput = 4 << 20
	// planTimeout is how long reading the plan may take; applying it is given as long as a deployment needs.
	planTimeout  = 2 * time.Minute
	applyTimeout = 30 * time.Minute
)

// splitCommandLine splits a command like a shell line without running one: spaces separate words, single or double quotes keep spaces
// in a word, a backslash outside quotes escapes the next character. An unclosed quote is an error.
func splitCommandLine(text string) ([]string, error) {
	var words []string
	var current strings.Builder
	inWord := false
	var quote rune
	escaped := false
	for _, r := range text {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
		case r == '\\':
			escaped, inWord = true, true
		case r == '\'' || r == '"':
			quote, inWord = r, true
		case r == ' ' || r == '\t':
			if inWord {
				words = append(words, current.String())
				current.Reset()
				inWord = false
			}
		default:
			current.WriteRune(r)
			inWord = true
		}
	}
	if quote != 0 || escaped {
		return nil, errors.New("the updater command has an unclosed quote or a trailing backslash")
	}
	if inWord {
		words = append(words, current.String())
	}
	return words, nil
}

// planRow and updaterPlan are the parts of the answer of `deploy-plan --json` that this command shows.
type planRow struct {
	Project         string  `json:"project"`
	State           string  `json:"state"`
	CheckoutVersion *string `json:"checkoutVersion"`
	DeviceVersion   *string `json:"deviceVersion"`
	Unit            *string `json:"unit"`
	Note            string  `json:"note"`
}

type updaterPlan struct {
	SchemaVersion string    `json:"schema_version"`
	Steps         int       `json:"steps"`
	Projects      []planRow `json:"projects"`
}

// limitedBuffer keeps at most maxUpdaterOutput bytes and drops the rest.
type limitedBuffer struct {
	buf       bytes.Buffer
	truncated bool
}

func (l *limitedBuffer) Write(p []byte) (int, error) {
	room := maxUpdaterOutput - l.buf.Len()
	if room <= 0 {
		l.truncated = true
		return len(p), nil
	}
	if len(p) > room {
		l.buf.Write(p[:room])
		l.truncated = true
		return len(p), nil
	}
	return l.buf.Write(p)
}

// runUpdater starts the updater command with the arguments and returns what it printed.
func runUpdater(command []string, extra []string, timeout time.Duration) (stdout, stderr []byte, err error) {
	args := append(append([]string{}, command[1:]...), extra...)
	cmd := exec.Command(command[0], args...)
	var out, errOut limitedBuffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Start(); err != nil {
		return nil, nil, newCliError(ExitGeneralError, fmt.Errorf("could not start the updater %q: %w", command[0], err))
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case waitErr := <-done:
		if out.truncated {
			return out.buf.Bytes(), errOut.buf.Bytes(), newCliError(ExitServerError, errors.New("the updater printed more than 4 MiB"))
		}
		return out.buf.Bytes(), errOut.buf.Bytes(), waitErr
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		<-done
		return out.buf.Bytes(), errOut.buf.Bytes(), newCliError(ExitNetworkError, fmt.Errorf("the updater did not finish in %s", timeout))
	}
}

// parsePlan reads and checks the plan the updater printed.
func parsePlan(raw []byte) (updaterPlan, error) {
	var plan updaterPlan
	if err := json.Unmarshal(raw, &plan); err != nil {
		return plan, newCliError(ExitServerError, fmt.Errorf("the updater did not print a plan: %w", err))
	}
	if plan.SchemaVersion != "1.0" {
		return plan, newCliError(ExitServerError, fmt.Errorf("the plan of the updater has the version %q, this command reads 1.0", printable(plan.SchemaVersion)))
	}
	steps := 0
	for _, row := range plan.Projects {
		if row.State == "deploy" {
			steps++
		}
	}
	if steps != plan.Steps {
		return plan, newCliError(ExitServerError, errors.New("the plan of the updater counts its steps wrongly"))
	}
	return plan, nil
}

func versionText(v *string) string {
	if v == nil || *v == "" {
		return "-"
	}
	return printable(*v)
}

// cmdDeploy implements `hydra-cli deploy`.
func cmdDeploy(w io.Writer, args []string) error {
	parsed, err := parseFlagSet("deploy", args,
		[]string{"--updater", "--workspace", "--device-root", "--install-plan", "--timeout"},
		[]string{"--apply", "--yes", "--json"})
	if err != nil {
		return err
	}
	if err := rejectExtraArgs("deploy", parsed.Rest); err != nil {
		return err
	}
	apply, confirmed, jsonOutput := parsed.Bool("--apply"), parsed.Bool("--yes"), parsed.Bool("--json")
	if apply && !confirmed {
		return newCliError(ExitUsageError, errors.New("deploy --apply restarts programs on the device: add --yes to do it (nothing was run)"))
	}
	if confirmed && !apply {
		return newCliError(ExitUsageError, errors.New("--yes only confirms --apply"))
	}
	line := parsed.String("--updater")
	if line == "" {
		line = os.Getenv(updaterEnv)
	}
	if line == "" {
		line = defaultUpdater
	}
	command, err := splitCommandLine(line)
	if err != nil {
		return newCliError(ExitUsageError, err)
	}
	if len(command) == 0 {
		return newCliError(ExitUsageError, errors.New("the updater command is empty"))
	}
	readTimeout, runTimeout := planTimeout, applyTimeout
	if parsed.String("--timeout") != "" {
		seconds, ok := parsePositiveSeconds(parsed.String("--timeout"))
		if !ok || seconds > time.Hour {
			return newCliError(ExitUsageError, errors.New("--timeout must be a number of seconds from 1 to 3600"))
		}
		readTimeout, runTimeout = seconds, seconds
	}
	extra := []string{"deploy-plan", "--json"}
	for _, option := range []string{"--workspace", "--device-root", "--install-plan"} {
		if value := parsed.String(option); value != "" {
			extra = append(extra, option, value)
		}
	}
	// first the plan, which only reads
	out, errOut, runErr := runUpdater(command, extra, readTimeout)
	if runErr != nil {
		return updaterFailure("reading the plan", runErr, errOut)
	}
	plan, err := parsePlan(out)
	if err != nil {
		return err
	}
	if !apply {
		return printDeployPlan(w, jsonOutput, plan, false, nil)
	}
	if plan.Steps == 0 {
		return printDeployPlan(w, jsonOutput, plan, true, []string{"nothing to deploy: the device runs what the workspace has"})
	}
	// then the same command with --apply --yes: the updater decides (switch, signature, health check, way back)
	_, applyErr, runErr := runUpdater(command, append(extra, "--apply", "--yes"), runTimeout)
	messages := strings.Split(strings.TrimSpace(string(applyErr)), "\n")
	if runErr != nil {
		_ = printDeployPlan(w, jsonOutput, plan, true, messages)
		return updaterFailure("deploying", runErr, applyErr)
	}
	return printDeployPlan(w, jsonOutput, plan, true, messages)
}

// updaterFailure turns a failed run into an error that carries the first line the updater said.
func updaterFailure(what string, runErr error, stderr []byte) error {
	var cli *CliError
	if errors.As(runErr, &cli) {
		return runErr
	}
	reason := strings.TrimSpace(printable(strings.SplitN(string(stderr), "\n", 2)[0]))
	if reason == "" {
		reason = runErr.Error()
	}
	return newCliError(ExitServerError, fmt.Errorf("the updater failed while %s: %s", what, reason))
}

func printDeployPlan(w io.Writer, jsonOutput bool, plan updaterPlan, applied bool, messages []string) error {
	if jsonOutput {
		rows := make([]map[string]any, 0, len(plan.Projects))
		for _, row := range plan.Projects {
			rows = append(rows, map[string]any{"project": row.Project, "state": row.State, "checkout_version": versionOrNil(row.CheckoutVersion), "device_version": versionOrNil(row.DeviceVersion), "unit": versionOrNil(row.Unit)})
		}
		return writeJSONResult(w, "deploy", true, map[string]any{"applied": applied, "steps": plan.Steps, "projects": rows, "messages": cleanMessages(messages)})
	}
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "PROJECT\tSTATE\tDEVICE\tWORKSPACE\tUNIT")
	for _, row := range plan.Projects {
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\n", printable(row.Project), printable(row.State), versionText(row.DeviceVersion), versionText(row.CheckoutVersion), versionText(row.Unit))
	}
	if err := table.Flush(); err != nil {
		return err
	}
	if applied {
		for _, message := range cleanMessages(messages) {
			fmt.Fprintln(w, message)
		}
	} else {
		fmt.Fprintf(w, "%d step(s) to deploy. Nothing was changed: add --apply --yes to deploy them.\n", plan.Steps)
	}
	return nil
}

func versionOrNil(v *string) any {
	if v == nil {
		return nil
	}
	return printable(*v)
}

func cleanMessages(messages []string) []string {
	var out []string
	for _, message := range messages {
		if trimmed := strings.TrimSpace(printable(message)); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
