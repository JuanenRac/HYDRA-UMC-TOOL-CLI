// =============================================================================
// HYDRA-UMC-TOOL-CLI - cmd/hydra-cli/fleet_test.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0-or-later - see LICENSE
// =============================================================================
package main

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

// hydraServerOn starts a fake server that answers /api/hydra-info like HYDRA-UMC-SERVER.
func hydraServerOn(t *testing.T, version string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/hydra-info" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"schema_version":"1.0","appVersion":"` + version + `","product":"HYDRA-UMC","hostname":"cm5-` + version + `","remoteApiVersion":2,"controllerCount":1,"robotCount":1}`))
	}))
	t.Cleanup(server.Close)
	return server
}

func portOf(t *testing.T, server *httptest.Server) int {
	t.Helper()
	_, port, err := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	n, _ := strconv.Atoi(port)
	return n
}

func TestScan_FindsAHydraServerAndSkipsEverythingElse(t *testing.T) {
	clearCredentials(t)
	hydra := hydraServerOn(t, "0.8.0")
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"hello":"world"}`)) }))
	defer other.Close()
	notFound := httptest.NewServer(http.NotFoundHandler())
	defer notFound.Close()

	ports := strconv.Itoa(portOf(t, hydra)) + "," + strconv.Itoa(portOf(t, other)) + "," + strconv.Itoa(portOf(t, notFound)) + ",1"
	var out bytes.Buffer
	if err := cmdScan(&out, []string{"127.0.0.1", "--ports", ports, "--probe-timeout-ms", "500"}); err != nil {
		t.Fatalf("cmdScan: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "127.0.0.1") || !strings.Contains(text, strconv.Itoa(portOf(t, hydra))) || !strings.Contains(text, "cm5-0.8.0") {
		t.Fatalf("the server was not found:\n%s", text)
	}
	if strings.Contains(text, strconv.Itoa(portOf(t, other))) || strings.Contains(text, strconv.Itoa(portOf(t, notFound))) {
		t.Fatalf("something that is not a HYDRA-UMC-SERVER was listed:\n%s", text)
	}
}

func TestScan_JSONIsAnArrayEvenWhenNothingIsFound(t *testing.T) {
	clearCredentials(t)
	var out bytes.Buffer
	if err := cmdScan(&out, []string{"127.0.0.1/32", "--ports", "1", "--json", "--probe-timeout-ms", "200"}); err != nil {
		t.Fatal(err)
	}
	var found []foundServer
	if err := json.Unmarshal(out.Bytes(), &found); err != nil || found == nil || len(found) != 0 {
		t.Fatalf("not an empty array: %q (%v)", out.String(), err)
	}
	out.Reset()
	if err := cmdScan(&out, []string{"127.0.0.1", "--ports", "1", "--probe-timeout-ms", "200"}); err != nil || !strings.Contains(out.String(), "no HYDRA-UMC-SERVER found") {
		t.Fatalf("%v %q", err, out.String())
	}
}

func TestScan_NeverSendsACredentialAndDoesNotFollowRedirects(t *testing.T) {
	clearCredentials(t)
	t.Setenv(tokenEnv, "secret-token")
	var sawAuth, redirectedHits int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { atomic.AddInt32(&redirectedHits, 1) }))
	defer target.Close()
	redirecting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			atomic.AddInt32(&sawAuth, 1)
		}
		http.Redirect(w, r, target.URL+"/api/hydra-info", http.StatusFound)
	}))
	defer redirecting.Close()
	var out bytes.Buffer
	if err := cmdScan(&out, []string{"127.0.0.1", "--ports", strconv.Itoa(portOf(t, redirecting)), "--probe-timeout-ms", "500"}); err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&sawAuth) != 0 || atomic.LoadInt32(&redirectedHits) != 0 {
		t.Fatalf("auth seen: %d, redirect followed: %d", sawAuth, redirectedHits)
	}
}

func TestParseScanTargets_ExpandsSkipsTheEdgesAndRefusesTheDangerous(t *testing.T) {
	addresses, err := parseScanTargets([]string{"192.168.1.0/30"}, 1024, false)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"192.168.1.1", "192.168.1.2"} // the network and the broadcast address are skipped
	if len(addresses) != 2 || addresses[0].String() != want[0] || addresses[1].String() != want[1] {
		t.Fatalf("addresses = %v", addresses)
	}
	if single, _ := parseScanTargets([]string{"10.0.0.7"}, 1024, false); len(single) != 1 || single[0] != netip.MustParseAddr("10.0.0.7") {
		t.Fatalf("single = %v", single)
	}
	cases := map[string]ExitCode{
		"8.8.8.0/24":     ExitRefused,    // public
		"10.0.0.0/8":     ExitRefused,    // too many
		"192.168.0.0/16": ExitRefused,    // 65534 > 1024
		"192.168.1.0/99": ExitUsageError, // not a prefix
		"not-an-address": ExitUsageError,
		"fd00::/64":      ExitRefused, // an IPv6 /64 is far too big
	}
	for target, wantCode := range cases {
		if _, err := parseScanTargets([]string{target}, 1024, false); exitCodeFor(err) != wantCode {
			t.Errorf("%s: %v, want exit %d", target, err, wantCode)
		}
	}
	// the public range is allowed only on request, and the host limit still holds
	if _, err := parseScanTargets([]string{"8.8.8.0/30"}, 1024, true); err != nil {
		t.Errorf("explicitly allowed public range: %v", err)
	}
	if got, err := parseScanTargets([]string{"10.0.0.0/16"}, 70000, false); err != nil || len(got) != 65534 {
		t.Errorf("a /16 within a raised limit: %d %v", len(got), err)
	}
	if _, err := parseScanTargets([]string{"192.168.1.0/24", "192.168.2.0/24", "192.168.3.0/24", "192.168.4.0/24"}, 600, false); exitCodeFor(err) != ExitRefused {
		t.Errorf("several networks that add up past the limit: %v", err)
	}
}

func TestScan_OptionErrors(t *testing.T) {
	clearCredentials(t)
	bad := [][]string{
		{},
		{"127.0.0.1", "--ports", "0"},
		{"127.0.0.1", "--ports", "70000"},
		{"127.0.0.1", "--ports", "a,b"},
		{"127.0.0.1", "--scheme", "ftp"},
		{"127.0.0.1", "--concurrency", "0"},
		{"127.0.0.1", "--max-hosts", "x"},
		{"127.0.0.1", "--probe-timeout-ms", "1"},
		{"127.0.0.1", "--nonsense"},
		{"8.8.8.8"},
	}
	for _, args := range bad {
		if err := cmdScan(&bytes.Buffer{}, args); err == nil || exitCodeFor(err) == ExitOK {
			t.Errorf("scan %v was accepted", args)
		}
	}
}

// ---- fleet doctor -----------------------------------------------------------

// doctorServer is a fake that passes the doctor (robots matches the count it reports).
func doctorServer(t *testing.T, version string, reportedRobots, actualRobots int) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/hydra-info":
			_, _ = w.Write([]byte(`{"schema_version":"1.0","appVersion":"` + version + `","controllerCount":1,"robotCount":` + strconv.Itoa(reportedRobots) + `}`))
		case "/api/settings":
			robots := make([]string, actualRobots)
			for i := range robots {
				robots[i] = `{"name":"R` + strconv.Itoa(i) + `"}`
			}
			_, _ = w.Write([]byte(`{"controllers":[{"name":"M","robots":[` + strings.Join(robots, ",") + `]}]}`))
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func writeList(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "servers.txt")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFleetDoctor_AllHealthyIsOneAnswerAndExitZero(t *testing.T) {
	clearCredentials(t)
	a, b := doctorServer(t, "0.8.0", 2, 2), doctorServer(t, "0.8.1", 1, 1)
	list := writeList(t, "# the cell", a.URL, "", b.URL+"/", a.URL) // a trailing slash and a repeat are folded
	var out bytes.Buffer
	if err := cmdFleet(&out, []string{"doctor", "--servers", list}); err != nil {
		t.Fatalf("cmdFleet: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "FLEET=PASS servers=2") || !strings.Contains(text, "0.8.0") || !strings.Contains(text, "0.8.1") {
		t.Fatalf("unexpected output:\n%s", text)
	}
}

func TestFleetDoctor_OneBrokenServerFailsTheFleetWithItsOwnCode(t *testing.T) {
	clearCredentials(t)
	good := doctorServer(t, "0.8.0", 1, 1)
	broken := doctorServer(t, "0.8.0", 5, 1) // reports 5 robots, holds 1: a server error
	list := writeList(t, good.URL, broken.URL)
	var out bytes.Buffer
	err := cmdFleet(&out, []string{"doctor", "--servers", list})
	if exitCodeFor(err) != ExitServerError {
		t.Fatalf("err = %v, want the exit code the only failure has", err)
	}
	if !strings.Contains(out.String(), "FLEET=FAIL servers=2 ok=1 failed=1") || !strings.Contains(out.String(), "robot count mismatch") {
		t.Fatalf("unexpected output:\n%s", out.String())
	}
}

func TestFleetDoctor_FailuresOfDifferentKindsExitOne(t *testing.T) {
	clearCredentials(t)
	broken := doctorServer(t, "0.8.0", 5, 1)
	unreachable := "http://127.0.0.1:1"
	list := writeList(t, broken.URL, unreachable)
	err := cmdFleet(&bytes.Buffer{}, []string{"doctor", "--servers", list, "--timeout", "1"})
	if exitCodeFor(err) != ExitGeneralError {
		t.Fatalf("err = %v, want exit 1 for failures of different kinds", err)
	}
}

func TestFleetDoctor_JSONHasOneReportPerServer(t *testing.T) {
	clearCredentials(t)
	a, b := doctorServer(t, "0.8.0", 1, 1), doctorServer(t, "0.8.0", 9, 1)
	list := writeList(t, a.URL, b.URL)
	var out bytes.Buffer
	_ = cmdFleet(&out, []string{"doctor", "--servers", list, "--json"})
	var results []fleetResult
	if err := json.Unmarshal(out.Bytes(), &results); err != nil || len(results) != 2 {
		t.Fatalf("%v: %s", err, out.String())
	}
	if !results[0].OK || results[1].OK || results[1].ExitCode != int(ExitServerError) || len(results[1].Report.Checks) == 0 {
		t.Fatalf("results = %+v", results)
	}
}

func TestFleetDoctor_ListAndOptionErrors(t *testing.T) {
	clearCredentials(t)
	bad := [][]string{
		{},
		{"status"},
		{"doctor"},
		{"doctor", "--servers", filepath.Join(t.TempDir(), "nope.txt")},
		{"doctor", "--servers", writeList(t, "# nothing")},
		{"doctor", "--servers", writeList(t, "not a url")},
		{"doctor", "--servers", writeList(t, "http://127.0.0.1:1"), "--concurrency", "0"},
		{"doctor", "--servers", writeList(t, "http://127.0.0.1:1"), "extra"},
	}
	for _, args := range bad {
		if err := cmdFleet(&bytes.Buffer{}, args); err == nil || exitCodeFor(err) == ExitOK {
			t.Errorf("fleet %v was accepted", args)
		}
	}
}
