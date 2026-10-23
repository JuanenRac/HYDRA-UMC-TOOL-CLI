// =============================================================================
// HYDRA-UMC-TOOL-CLI - cmd/hydra-cli/control_test.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0 - see LICENSE
// =============================================================================
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type recorded struct {
	Method, Path, Controller, Query string
	Body                            map[string]any
}

// fakeServer answers by path and records every request it saw.
func fakeServer(t *testing.T, handler func(r *http.Request, rec recorded) (int, string)) (*httptest.Server, *[]recorded) {
	t.Helper()
	var mu sync.Mutex
	seen := []recorded{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		rec := recorded{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Controller: r.Header.Get("X-Controller-Id")}
		_ = json.Unmarshal(raw, &rec.Body)
		mu.Lock()
		seen = append(seen, rec)
		mu.Unlock()
		status, body := handler(r, rec)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &seen
}

func TestResolveControllerID(t *testing.T) {
	t.Setenv(controllerEnv, "")
	id, err := resolveControllerID("")
	if err != nil || !controllerIDPattern.MatchString(id) || !strings.HasPrefix(id, "hydra-cli-") {
		t.Fatalf("default id = %q, %v", id, err)
	}
	t.Setenv(controllerEnv, "from-env")
	if id, _ := resolveControllerID(""); id != "from-env" {
		t.Fatalf("env id = %q", id)
	}
	if id, _ := resolveControllerID("from-flag"); id != "from-flag" {
		t.Fatalf("flag id = %q", id)
	}
	if _, err := resolveControllerID("bad id!"); exitCodeFor(err) != ExitUsageError {
		t.Fatalf("a malformed id must be a usage error, got %v", err)
	}
	if _, err := resolveControllerID(strings.Repeat("a", 65)); err == nil {
		t.Fatal("65 characters must be refused")
	}
}

func TestClaimSendsControllerIDInBodyAndHeader(t *testing.T) {
	srv, seen := fakeServer(t, func(r *http.Request, rec recorded) (int, string) { return 200, `{"success":true}` })
	var out bytes.Buffer
	if err := cmdClaim(&out, []string{"--server", srv.URL, "--controller-id", "cli-1", "3", "--ttl-ms", "60000", "--reason", "bench"}); err != nil {
		t.Fatal(err)
	}
	got := (*seen)[0]
	if got.Path != "/api/robot/3/claim" || got.Controller != "cli-1" || got.Body["controllerId"] != "cli-1" || got.Body["ttlMs"] != float64(60000) || got.Body["reason"] != "bench" {
		t.Fatalf("unexpected claim request: %+v", got)
	}
	if !strings.Contains(out.String(), "reserved") {
		t.Fatalf("output: %s", out.String())
	}
}

func TestReleaseSendsControllerID(t *testing.T) {
	srv, seen := fakeServer(t, func(r *http.Request, rec recorded) (int, string) { return 200, `{"success":true}` })
	if err := cmdRelease(io.Discard, []string{"--server", srv.URL, "--controller-id", "cli-1", "3"}); err != nil {
		t.Fatal(err)
	}
	got := (*seen)[0]
	if got.Path != "/api/robot/3/release" || got.Body["controllerId"] != "cli-1" {
		t.Fatalf("unexpected release request: %+v", got)
	}
}

func TestRefusalsAreDistinctAndExitRefused(t *testing.T) {
	cases := []struct {
		status int
		body   string
		want   string
	}{
		{409, `{"error":"robot is already claimed by ana","code":"other-account"}`, "another account"},
		{409, `{"error":"x","code":"other-controller"}`, "another window or device"},
		{409, `{"error":"x","code":"controller-id-required"}`, "named none"},
		{400, `{"error":"x","code":"controller-id-required"}`, "requires the request to name its controller"},
		{403, `{"error":"x","code":"jog-not-armed"}`, "jog-arm"},
		{409, `{"error":"claimed"}`, "is reserved"},
	}
	for _, c := range cases {
		srv, _ := fakeServer(t, func(r *http.Request, rec recorded) (int, string) { return c.status, c.body })
		err := cmdClaim(io.Discard, []string{"--server", srv.URL, "1"})
		if err == nil || exitCodeFor(err) != ExitRefused || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("status %d body %s: err = %v (code %d), want %q", c.status, c.body, err, exitCodeFor(err), c.want)
		}
	}
}

func TestExpiredSessionIsAuthError(t *testing.T) {
	srv, _ := fakeServer(t, func(r *http.Request, rec recorded) (int, string) { return 401, `{"error":"expired"}` })
	if err := cmdRelease(io.Discard, []string{"--server", srv.URL, "1"}); exitCodeFor(err) != ExitAuthError {
		t.Fatalf("want auth error, got %v", err)
	}
}

func TestClaimsListsReservations(t *testing.T) {
	expires := itoa(time.Now().Add(5 * time.Minute).UnixMilli())
	body := `{"claims":[{"robotId":1,"robotName":"Robot A1","cellId":"c","ownerId":"u","ownerUsername":"ana","controllerId":"cli-1","claimedAt":1,"expiresAt":` + expires + `},{"robotId":2,"robotName":"B","ownerUsername":"bob","controllerId":null,"expiresAt":` + expires + `}]}`
	srv, _ := fakeServer(t, func(r *http.Request, rec recorded) (int, string) { return 200, body })
	var out bytes.Buffer
	if err := cmdClaims(&out, []string{"--server", srv.URL, "--controller-id", "cli-1"}); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	for _, want := range []string{"Robot A1", "ana", "cli-1 (this program)", "bob", "not named"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in:\n%s", want, s)
		}
	}
	var js bytes.Buffer
	if err := cmdClaims(&js, []string{"--server", srv.URL, "--json"}); err != nil || !strings.Contains(js.String(), `"robotId": 1`) {
		t.Fatalf("json output: %v %s", err, js.String())
	}
}

func TestClaimsEmpty(t *testing.T) {
	srv, _ := fakeServer(t, func(r *http.Request, rec recorded) (int, string) { return 200, `{"claims":[]}` })
	var out bytes.Buffer
	if err := cmdClaims(&out, []string{"--server", srv.URL}); err != nil || !strings.Contains(out.String(), "No robot is reserved") {
		t.Fatalf("%v %s", err, out.String())
	}
}

const limitsBody = `{"limits":{"jog":{"armRequired":true,"armTtlMs":60000,"default":{"maxAmount":100,"maxAbsolute":1000},"models":{}},"temperature":{"default":{"bedMaxC":110},"models":{}},"claims":{"requireControllerId":false}},"defaults":{},"hardLimits":{},"robot":{"id":1,"model":"Parol6 (6-DOF)","jog":{"maxAmount":20,"maxAbsolute":500},"temperature":{"bedMaxC":90},"jogArmedUntil":null}}`

func TestLimitsPrintsRobotValues(t *testing.T) {
	srv, seen := fakeServer(t, func(r *http.Request, rec recorded) (int, string) { return 200, limitsBody })
	var out bytes.Buffer
	if err := cmdLimits(&out, []string{"--server", srv.URL, "--robot", "1"}); err != nil {
		t.Fatal(err)
	}
	if (*seen)[0].Query != "robotId=1" {
		t.Fatalf("query = %q", (*seen)[0].Query)
	}
	for _, want := range []string{"Jog must be armed:       yes", "relative jog, at most: 20", "bed target, at most:   90 C", "not armed"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in:\n%s", want, out.String())
		}
	}
}

func TestJogArmOnAndOff(t *testing.T) {
	exp := itoa(time.Now().Add(time.Minute).UnixMilli())
	srv, seen := fakeServer(t, func(r *http.Request, rec recorded) (int, string) {
		if rec.Body["armed"] == true {
			return 200, `{"success":true,"armed":true,"expiresAt":` + exp + `}`
		}
		return 200, `{"success":true,"armed":false}`
	})
	var out bytes.Buffer
	if err := cmdJogArm(&out, []string{"--server", srv.URL, "--controller-id", "cli-1", "1", "on", "--ttl-ms", "5000"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "armed for") || (*seen)[0].Body["ttlMs"] != float64(5000) || (*seen)[0].Body["controllerId"] != "cli-1" {
		t.Fatalf("%s %+v", out.String(), (*seen)[0])
	}
	out.Reset()
	if err := cmdJogArm(&out, []string{"--server", srv.URL, "1", "off"}); err != nil || !strings.Contains(out.String(), "disarmed") {
		t.Fatalf("%v %s", err, out.String())
	}
	if err := cmdJogArm(io.Discard, []string{"--server", srv.URL, "1", "maybe"}); exitCodeFor(err) != ExitUsageError {
		t.Fatalf("want usage error, got %v", err)
	}
}

func TestJogAboveLimitIsNotSent(t *testing.T) {
	srv, seen := fakeServer(t, func(r *http.Request, rec recorded) (int, string) { return 200, limitsBody })
	err := cmdJog(io.Discard, []string{"--server", srv.URL, "1", "x", "50"})
	if exitCodeFor(err) != ExitRefused || !strings.Contains(err.Error(), "nothing was sent") {
		t.Fatalf("err = %v", err)
	}
	for _, r := range *seen {
		if strings.HasSuffix(r.Path, "/command") {
			t.Fatal("a jog above the limit must not reach the server")
		}
	}
	// the absolute range is separate
	if err := cmdJog(io.Discard, []string{"--server", srv.URL, "1", "x", "600", "--absolute"}); exitCodeFor(err) != ExitRefused {
		t.Fatalf("absolute above 500 must be refused, got %v", err)
	}
}

func TestJogWithinLimitSendsControllerID(t *testing.T) {
	srv, seen := fakeServer(t, func(r *http.Request, rec recorded) (int, string) {
		if strings.HasSuffix(rec.Path, "/command") {
			return 200, `{"success":true}`
		}
		return 200, limitsBody
	})
	var out bytes.Buffer
	if err := cmdJog(&out, []string{"--server", srv.URL, "--controller-id", "cli-1", "1", "X", "-15.5"}); err != nil {
		t.Fatal(err)
	}
	cmd := (*seen)[len(*seen)-1]
	params := cmd.Body["params"].(map[string]any)
	if cmd.Controller != "cli-1" || cmd.Body["controllerId"] != "cli-1" || cmd.Body["command"] != "jog" || params["axis"] != "x" || params["amount"] != -15.5 {
		t.Fatalf("unexpected command: %+v", cmd)
	}
}

func TestJogNotArmedPointsToJogArm(t *testing.T) {
	srv, _ := fakeServer(t, func(r *http.Request, rec recorded) (int, string) {
		if strings.HasSuffix(rec.Path, "/command") {
			return 403, `{"error":"not armed","code":"jog-not-armed"}`
		}
		return 200, limitsBody
	})
	err := cmdJog(io.Discard, []string{"--server", srv.URL, "1", "x", "5"})
	if exitCodeFor(err) != ExitRefused || !strings.Contains(err.Error(), "jog-arm") {
		t.Fatalf("err = %v", err)
	}
}

func TestJogWarningsAreShownAndFail(t *testing.T) {
	srv, _ := fakeServer(t, func(r *http.Request, rec recorded) (int, string) {
		if strings.HasSuffix(rec.Path, "/command") {
			return 200, `{"success":true,"warnings":[{"field":"jog","message":"not applied"}]}`
		}
		return 200, limitsBody
	})
	var out bytes.Buffer
	err := cmdJog(&out, []string{"--server", srv.URL, "1", "x", "5"})
	if exitCodeFor(err) != ExitRefused || !strings.Contains(out.String(), "Warning (jog): not applied") {
		t.Fatalf("%v %s", err, out.String())
	}
}

func TestJogDoesNotSendWhenLimitsUnreadable(t *testing.T) {
	srv, seen := fakeServer(t, func(r *http.Request, rec recorded) (int, string) { return 500, `{}` })
	if err := cmdJog(io.Discard, []string{"--server", srv.URL, "1", "x", "5"}); err == nil {
		t.Fatal("must fail")
	}
	if len(*seen) != 1 {
		t.Fatalf("only the limits read may be sent, saw %d requests", len(*seen))
	}
}

func TestJogArgumentValidation(t *testing.T) {
	for _, args := range [][]string{{"1", "q", "5"}, {"1", "x", "abc"}, {"1", "x"}, {"one", "x", "5"}, {"1", "x", "NaN"}} {
		if err := cmdJog(io.Discard, append([]string{"--server", "http://127.0.0.1:1"}, args...)); exitCodeFor(err) != ExitUsageError {
			t.Fatalf("%v: want usage error, got %v", args, err)
		}
	}
}

func TestRunDispatchesControlCommands(t *testing.T) {
	for _, cmd := range []string{"claims", "claim", "release", "limits", "jog-arm", "jog"} {
		if got := run([]string{cmd, "--server", "http://127.0.0.1:1"}); got == ExitOK {
			t.Fatalf("%s without a reachable server must not be OK", cmd)
		}
	}
}

func itoa(v int64) string {
	b, _ := json.Marshal(v)
	return string(b)
}
