// =============================================================================
// HYDRA-UMC-TOOL-CLI - cmd/hydra-cli/flagset_commands_test.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0 - see LICENSE
// =============================================================================
// The options of release, limits, jog, table, scan, fleet doctor, plan-simulate and login are read by each command's own
// FlagSet declaration. These tests pin how they have always behaved: options and positionals in any order, the same message and
// exit code for a token that is not an option of the command, a missing value is a usage error, the last occurrence wins, and a
// value may look like an option.
package main

import (
	"bytes"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func wantUsage(t *testing.T, err error, message string) {
	t.Helper()
	if err == nil || exitCodeFor(err) != ExitUsageError || !strings.Contains(err.Error(), message) {
		t.Fatalf("want a usage error containing %q, got %v (exit %v)", message, err, exitCodeFor(err))
	}
}

func TestReleaseOptionsInAnyOrderLastWins(t *testing.T) {
	srv, seen := fakeServer(t, func(r *http.Request, rec recorded) (int, string) { return 200, `{"success":true}` })
	for _, args := range [][]string{
		{"--server", srv.URL, "3", "--reason", "first", "--reason", "done"},
		{"--reason", "first", "--reason", "done", "3", "--server", srv.URL},
		{"--reason", "first", "--server", srv.URL, "--reason", "done", "3"},
	} {
		if err := cmdRelease(io.Discard, args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		got := (*seen)[len(*seen)-1]
		if got.Path != "/api/robot/3/release" || got.Body["reason"] != "done" {
			t.Fatalf("%v: %+v", args, got)
		}
	}
	// a value may look like an option
	if err := cmdRelease(io.Discard, []string{"--server", srv.URL, "3", "--reason", "--force"}); err != nil {
		t.Fatal(err)
	}
	if got := (*seen)[len(*seen)-1]; got.Body["reason"] != "--force" {
		t.Fatalf("reason = %v", got.Body["reason"])
	}
}

func TestReleaseUnknownTokenAndMissingValue(t *testing.T) {
	srv, seen := fakeServer(t, func(r *http.Request, rec recorded) (int, string) { return 200, `{"success":true}` })
	wantUsage(t, cmdRelease(io.Discard, []string{"--server", srv.URL, "3", "--bogus"}), "usage: hydra-cli release <robotId>")
	wantUsage(t, cmdRelease(io.Discard, []string{"--server", srv.URL}), "usage: hydra-cli release <robotId>")
	wantUsage(t, cmdRelease(io.Discard, []string{"--server", srv.URL, "3", "--reason"}), "--reason needs a value")
	wantUsage(t, cmdRelease(io.Discard, []string{"--server", srv.URL, "3", "--if-match", "bad value!"}), "--if-match needs a revision")
	if len(*seen) != 0 {
		t.Fatalf("%d request(s) were sent for bad arguments", len(*seen))
	}
}

func TestLimitsOptionsInAnyOrderLastWins(t *testing.T) {
	srv, seen := fakeServer(t, func(r *http.Request, rec recorded) (int, string) { return 200, limitsBody })
	for _, args := range [][]string{
		{"--server", srv.URL, "--robot", "7", "--robot", "1", "--json"},
		{"--json", "--robot", "7", "--server", srv.URL, "--robot", "1"},
	} {
		var out bytes.Buffer
		if err := cmdLimits(&out, args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if got := (*seen)[len(*seen)-1].Query; got != "robotId=1" {
			t.Fatalf("%v: query %q", args, got)
		}
		if !strings.Contains(out.String(), `"robot"`) {
			t.Fatalf("%v: not JSON:\n%s", args, out.String())
		}
	}
}

func TestLimitsUnknownTokenAndMissingValue(t *testing.T) {
	srv, seen := fakeServer(t, func(r *http.Request, rec recorded) (int, string) { return 200, limitsBody })
	wantUsage(t, cmdLimits(io.Discard, []string{"--server", srv.URL, "--bogus"}), "limits does not accept arguments: --bogus")
	wantUsage(t, cmdLimits(io.Discard, []string{"--server", srv.URL, "1"}), "limits does not accept arguments: 1")
	wantUsage(t, cmdLimits(io.Discard, []string{"--server", srv.URL, "--robot"}), "--robot needs a value")
	// --json is taken before --robot, so it is never the value of --robot
	wantUsage(t, cmdLimits(io.Discard, []string{"--server", srv.URL, "--robot", "--json"}), "--robot needs a value")
	if len(*seen) != 0 {
		t.Fatalf("%d request(s) were sent for bad arguments", len(*seen))
	}
}

func TestJogOptionsAnywhereAndNegativeAmount(t *testing.T) {
	srv, seen := fakeServer(t, func(r *http.Request, rec recorded) (int, string) {
		if strings.HasSuffix(rec.Path, "/command") {
			return 200, `{"success":true}`
		}
		return 200, limitsBody
	})
	for _, args := range [][]string{
		{"--server", srv.URL, "1", "y", "-3", "--absolute"},
		{"--absolute", "--server", srv.URL, "1", "y", "-3"},
		{"1", "--absolute", "y", "--server", srv.URL, "-3"},
	} {
		if err := cmdJog(io.Discard, args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		cmd := (*seen)[len(*seen)-1]
		params, _ := cmd.Body["params"].(map[string]any)
		if params["axis"] != "y" || params["amount"] != float64(-3) || params["absolute"] != true {
			t.Fatalf("%v: %+v", args, cmd)
		}
	}
}

func TestJogUnknownTokenAndMissingValue(t *testing.T) {
	srv, seen := fakeServer(t, func(r *http.Request, rec recorded) (int, string) { return 200, limitsBody })
	wantUsage(t, cmdJog(io.Discard, []string{"--server", srv.URL, "1", "x", "5", "--bogus"}), "usage: hydra-cli jog <robotId> <axis> <amount>")
	wantUsage(t, cmdJog(io.Discard, []string{"--server", srv.URL, "1", "x", "5", "--if-match"}), "--if-match needs a value")
	if len(*seen) != 0 {
		t.Fatalf("%d request(s) were sent for bad arguments", len(*seen))
	}
}

func TestJogArmOptionsAnywhereLastWins(t *testing.T) {
	srv, seen := fakeServer(t, func(r *http.Request, rec recorded) (int, string) {
		return 200, `{"success":true,"armed":true,"expiresAt":4102444800000}`
	})
	for _, args := range [][]string{
		{"--server", srv.URL, "1", "on", "--ttl-ms", "1000", "--ttl-ms", "5000"},
		{"--ttl-ms", "1000", "1", "--ttl-ms", "5000", "on", "--server", srv.URL},
	} {
		if err := cmdJogArm(io.Discard, args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if got := (*seen)[len(*seen)-1]; got.Body["ttlMs"] != float64(5000) {
			t.Fatalf("%v: %+v", args, got)
		}
	}
	wantUsage(t, cmdJogArm(io.Discard, []string{"--server", srv.URL, "1", "on", "--bogus"}), "usage: hydra-cli jog-arm")
	wantUsage(t, cmdJogArm(io.Discard, []string{"--server", srv.URL, "1", "on", "--ttl-ms"}), "--ttl-ms needs a value")
}

func TestTableOptionsInAnyOrderLastWins(t *testing.T) {
	var sent map[string]any
	server, _ := ifMatchServer(t, func(r *http.Request, rec recorded) (int, string) {
		if strings.HasPrefix(r.URL.Path, "/api/safety-limits") {
			return 200, limitsAnswer
		}
		sent = rec.Body
		return 200, `{"success":true,"applied":{"x":1,"y":2}}`
	})
	for _, args := range [][]string{
		{"--server", server, "3", "5", "-5", "--rz", "90", "--rz", "10", "--relative"},
		{"--relative", "--rz", "90", "3", "--server", server, "5", "--rz", "10", "-5"},
	} {
		if err := cmdTable(io.Discard, args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		params, _ := sent["params"].(map[string]any)
		if params["rz"] != float64(10) || params["mode"] != "relative" || params["x"] != float64(5) || params["y"] != float64(-5) {
			t.Fatalf("%v: %v", args, sent)
		}
	}
}

func TestTableUnknownTokenAndMissingValue(t *testing.T) {
	server, _ := ifMatchServer(t, func(r *http.Request, rec recorded) (int, string) { return 200, limitsAnswer })
	wantUsage(t, cmdTable(io.Discard, []string{"--server", server, "3", "1", "2", "--bogus"}), "usage: hydra-cli table <robotId> <x> <y>")
	wantUsage(t, cmdTable(io.Discard, []string{"--server", server, "3", "1", "2", "--rz"}), "--rz needs a value")
}

func TestScanOptionsAnywhereLastWinsAndUnknownOption(t *testing.T) {
	wantUsage(t, cmdScan(io.Discard, []string{"127.0.0.1", "--bogus"}), "scan does not know the option --bogus")
	wantUsage(t, cmdScan(io.Discard, []string{"--bogus", "127.0.0.1"}), "scan does not know the option --bogus")
	wantUsage(t, cmdScan(io.Discard, []string{"--json"}), "usage: hydra-cli scan")
	wantUsage(t, cmdScan(io.Discard, []string{"127.0.0.1", "--ports"}), "--ports needs a value")
	// the last --scheme wins, in either order, and a value may look like an option
	wantUsage(t, cmdScan(io.Discard, []string{"--scheme", "https", "127.0.0.1", "--scheme", "ftp"}), `--scheme must be http or https, got "ftp"`)
	wantUsage(t, cmdScan(io.Discard, []string{"--scheme", "--json", "127.0.0.1"}), `got "--json"`)
	var out bytes.Buffer
	if err := cmdScan(&out, []string{"--scheme", "ftp", "--json", "127.0.0.1/32", "--scheme", "https", "--ports", "1", "--probe-timeout-ms", "200"}); err != nil {
		t.Fatalf("the last --scheme must win: %v", err)
	}
	if !strings.Contains(out.String(), "[") {
		t.Fatalf("--json anywhere must give JSON:\n%s", out.String())
	}
}

func TestFleetDoctorOptionsAnywhereLastWinsAndUnknownToken(t *testing.T) {
	clearCredentials(t)
	srv := doctorServer(t, "0.8.0", 1, 1)
	list := writeList(t, srv.URL)
	wantUsage(t, cmdFleet(io.Discard, []string{"doctor", "--servers", list, "--bogus"}), "fleet doctor does not accept arguments: --bogus")
	wantUsage(t, cmdFleet(io.Discard, []string{"doctor", "--servers"}), "--servers needs a value")
	wantUsage(t, cmdFleet(io.Discard, []string{"doctor", "--json"}), "fleet doctor needs --servers FILE")
	wantUsage(t, cmdFleet(io.Discard, []string{"doctor", "--servers", list, "--concurrency", "2", "--concurrency", "0"}), "--concurrency")
	for _, args := range [][]string{
		{"doctor", "--servers", list, "--concurrency", "0", "--concurrency", "2"},
		{"doctor", "--concurrency", "0", "--json", "--concurrency", "2", "--servers", list},
	} {
		var out bytes.Buffer
		if err := cmdFleet(&out, args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
}

func TestPlanSimulateOptionsAnywhereLastWinsAndUnknownToken(t *testing.T) {
	planner, _ := answerServer(t, 200, acceptedVerdict)
	good, other := writePlan(t, planBody), writePlan(t, `not json`)
	for _, args := range [][]string{
		{"--plan", other, "--planner", planner, "--plan", good, "--json"},
		{"--json", "--planner", planner, "--plan", other, "--plan", good},
	} {
		var out bytes.Buffer
		if err := cmdPlanSimulate(&out, nil, args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if !strings.Contains(out.String(), "accepted") {
			t.Fatalf("%v:\n%s", args, out.String())
		}
	}
	wantUsage(t, cmdPlanSimulate(io.Discard, nil, []string{"--plan", good, "--planner", planner, "--bogus"}), "plan-simulate does not accept arguments: --bogus")
	wantUsage(t, cmdPlanSimulate(io.Discard, nil, []string{"--plan"}), "--plan needs a value")
	wantUsage(t, cmdPlanSimulate(io.Discard, nil, []string{"--plan", good, "--planner"}), "--planner needs a value")
}

func TestLoginOptionsAnywhereLastWinsAndUnknownToken(t *testing.T) {
	useTempSession(t)
	server := newSessionServer(t)
	t.Setenv(userEnv, "")
	t.Setenv(passwordEnv, "")
	wantUsage(t, cmdLogin(io.Discard, strings.NewReader(""), []string{"--server", server.URL, "extra"}), "login does not accept arguments: extra")
	wantUsage(t, cmdLogin(io.Discard, strings.NewReader(""), []string{"--server", server.URL, "--user"}), "--user needs a value")
	if err := cmdLogin(io.Discard, strings.NewReader("right-password\n"), []string{"--password-stdin", "--user", "nobody", "--server", server.URL, "--user", "operator"}); err != nil {
		t.Fatalf("the last --user must win: %v", err)
	}
}

func TestParseFlagSetOrderedEarlierOptionsTakeTheirValuesFirst(t *testing.T) {
	// listed first, --reason keeps "--json" as its value; listed second, --json is taken first and --reason has no value
	p, err := parseFlagSetOrdered("x", []string{"--reason", "--json"}, valueOpt("--reason"), switchOpt("--json"))
	if err != nil || p.String("--reason") != "--json" || p.Bool("--json") || len(p.Rest) != 0 {
		t.Fatalf("value first: %v %q %v %v", err, p.String("--reason"), p.Bool("--json"), p.Rest)
	}
	_, err = parseFlagSetOrdered("x", []string{"--reason", "--json"}, switchOpt("--json"), valueOpt("--reason"))
	wantUsage(t, err, "--reason needs a value")
	p, err = parseFlagSetOrdered("x", []string{"a", "--n", "1", "--n", "2", "--bogus", "b"}, valueOpt("--n"))
	if err != nil || p.String("--n") != "2" || !p.Seen("--n") || !reflect.DeepEqual(p.Rest, []string{"a", "--bogus", "b"}) {
		t.Fatalf("last wins: %v %q %v", err, p.String("--n"), p.Rest)
	}
}

func TestMetricsConnectionsAndManifestOptions(t *testing.T) {
	wantUsage(t, cmdMetrics(io.Discard, []string{"--server", "http://127.0.0.1:1", "--bogus"}), "metrics does not accept arguments: --bogus")
	wantUsage(t, cmdConnections(io.Discard, []string{"--server", "http://127.0.0.1:1", "events", "--limit"}), "--limit needs a value")
	wantUsage(t, cmdConnections(io.Discard, []string{"--server", "http://127.0.0.1:1", "--kind", "nope", "--kind", "bogus"}), `got "bogus"`)
	wantUsage(t, cmdConnections(io.Discard, []string{"--server", "http://127.0.0.1:1", "clients", "--limit", "5"}), "belong to `connections events`")
	wantUsage(t, cmdManifest(io.Discard, []string{"validate", "--bogus"}), "manifest validate does not know the option --bogus")
	wantUsage(t, cmdManifest(io.Discard, []string{"validate", "--schema"}), "--schema needs a value")
}
