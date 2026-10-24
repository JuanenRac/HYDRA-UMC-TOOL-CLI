// =============================================================================
// HYDRA-UMC-TOOL-CLI - Reading the metrics of the server: cmd/hydra-cli/metrics.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0-or-later - see LICENSE
// =============================================================================
// `hydra-cli metrics` reads GET /metrics (the Prometheus text the server exposes) and shows the figures an
// operator looks at first: uptime, connected clients, commands sent by kind, login failures by reason and
// the temperature and load of the host. --all prints every sample; --json prints them as JSON.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
)

// sample is one line of a Prometheus text page.
type sample struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels,omitempty"`
	Value  float64           `json:"value"`
}

// parseLabels reads the inside of {a="1",b="2"}; a value may hold escaped quotes and backslashes.
func parseLabels(text string) (map[string]string, bool) {
	labels := map[string]string{}
	for len(text) > 0 {
		eq := strings.IndexByte(text, '=')
		if eq <= 0 || eq+1 >= len(text) || text[eq+1] != '"' {
			return nil, false
		}
		name := strings.TrimSpace(text[:eq])
		var value strings.Builder
		i := eq + 2
		closed := false
		for ; i < len(text); i++ {
			switch text[i] {
			case '\\':
				if i+1 < len(text) {
					i++
					switch text[i] {
					case 'n':
						value.WriteByte('\n')
					default:
						value.WriteByte(text[i])
					}
				}
			case '"':
				closed = true
			default:
				value.WriteByte(text[i])
			}
			if closed {
				break
			}
		}
		if !closed {
			return nil, false
		}
		labels[name] = value.String()
		text = strings.TrimLeft(text[i+1:], ", ")
	}
	return labels, true
}

// parseMetrics reads a Prometheus text page. Comment lines are skipped; a line that is not a sample is an error.
func parseMetrics(text string) ([]sample, error) {
	var samples []sample
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 64*1024), 1<<20)
	for lineNo := 1; scanner.Scan(); lineNo++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var name, rest string
		var labels map[string]string
		if brace := strings.IndexByte(line, '{'); brace >= 0 {
			end := strings.LastIndexByte(line, '}')
			if end < brace {
				return nil, fmt.Errorf("line %d is not a metric sample", lineNo)
			}
			parsed, ok := parseLabels(line[brace+1 : end])
			if !ok {
				return nil, fmt.Errorf("line %d has labels that cannot be read", lineNo)
			}
			name, labels, rest = line[:brace], parsed, strings.TrimSpace(line[end+1:])
		} else {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return nil, fmt.Errorf("line %d is not a metric sample", lineNo)
			}
			name, rest = fields[0], strings.Join(fields[1:], " ")
		}
		valueText := strings.Fields(rest)
		if name == "" || len(valueText) == 0 {
			return nil, fmt.Errorf("line %d is not a metric sample", lineNo)
		}
		value, err := strconv.ParseFloat(valueText[0], 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: %q is not a number", lineNo, valueText[0])
		}
		samples = append(samples, sample{Name: name, Labels: labels, Value: value})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return samples, nil
}

// keyMetrics are the figures the summary shows, in this order.
var keyMetrics = []struct{ name, label string }{
	{"hydra_process_uptime_seconds", "uptime (s)"},
	{"hydra_ws_clients_connected", "WebSocket clients"},
	{"hydra_robot_commands_total", "commands sent"},
	{"hydra_auth_failures_total", "login and token failures"},
	{"hydra_system_cpu_load", "CPU load"},
	{"hydra_system_memory_usage_percent", "memory used (%)"},
	{"hydra_system_temp_celsius", "temperature (C)"},
}

func labelText(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+printable(labels[key]))
	}
	return strings.Join(parts, " ")
}

// cmdMetrics implements `hydra-cli metrics [--all] [--json]`.
func cmdMetrics(w io.Writer, args []string) error {
	server, timeout, rest, err := resolveTarget(args)
	if err != nil {
		return err
	}
	all, rest := takeSwitch(rest, "--all")
	jsonOutput, rest := takeSwitch(rest, "--json")
	if err := rejectExtraArgs("metrics", rest); err != nil {
		return err
	}
	client := newClient(timeout)
	status, body, err := getAuthenticated(&client, server, "/metrics")
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return newCliError(ExitServerError, fmt.Errorf("%s/metrics replied with HTTP %d: %s", server, status, printable(string(body))))
	}
	samples, err := parseMetrics(string(body))
	if err != nil {
		return newCliError(ExitServerError, fmt.Errorf("unexpected metrics from %s: %w", server, err))
	}
	if jsonOutput {
		if samples == nil {
			samples = []sample{}
		}
		encoded, err := json.MarshalIndent(samples, "", "  ")
		if err != nil {
			return newCliError(ExitGeneralError, err)
		}
		_, err = fmt.Fprintln(w, string(encoded))
		return err
	}
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if all {
		fmt.Fprintln(table, "METRIC\tLABELS\tVALUE")
		for _, s := range samples {
			fmt.Fprintf(table, "%s\t%s\t%s\n", printable(s.Name), labelText(s.Labels), strconv.FormatFloat(s.Value, 'g', -1, 64))
		}
		return table.Flush()
	}
	fmt.Fprintln(table, "FIGURE\tLABELS\tVALUE")
	shown := 0
	for _, key := range keyMetrics {
		for _, s := range samples {
			if s.Name == key.name {
				fmt.Fprintf(table, "%s\t%s\t%s\n", key.label, labelText(s.Labels), strconv.FormatFloat(s.Value, 'g', -1, 64))
				shown++
			}
		}
	}
	if shown == 0 {
		fmt.Fprintf(w, "%s answered, but none of the usual figures is in it (%d samples; --all shows them)\n", server, len(samples))
		return nil
	}
	return table.Flush()
}
