// =============================================================================
// HYDRA-UMC-TOOL-CLI - cmd/hydra-cli/observe_test.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0-or-later - see LICENSE
// =============================================================================
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// answerServer answers every request with one status and body and remembers the requests it got.
type seenRequest struct {
	Method, Path, Query, Authorization string
	Body                               string
}

func answerServer(t *testing.T, status int, body string) (string, func() []seenRequest) {
	t.Helper()
	var mu sync.Mutex
	var seen []seenRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		seen = append(seen, seenRequest{r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Get("Authorization"), string(raw)})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv.URL, func() []seenRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]seenRequest(nil), seen...)
	}
}

// runBoth returns the exit code, the stdout and the stderr of a command line.
func runBoth(t *testing.T, args ...string) (ExitCode, string, string) {
	t.Helper()
	var code ExitCode
	var stderr string
	stdout := captureStdout(t, func() { stderr = captureStderr(t, func() { code = run(args) }) })
	return code, stdout, stderr
}

func signIn_t(t *testing.T) {
	t.Helper()
	clearCredentials(t)
	t.Setenv(tokenEnv, "observe-test-token")
}

const clientsBody = `{"clients":[
 {"schema_version":"1.0","id":"c1","kind":"studio","user":"ana","app_version":"0.9.1","origin":"local","two_factor":true,"first_seen_utc":"2026-01-01T00:00:00Z","last_seen_utc":"2026-01-02T00:00:00Z","active_sessions":1,"logins_ok":4,"logins_failed":0},
 {"schema_version":"1.0","id":"c2","kind":"android","first_seen_utc":"2026-01-01T00:00:00Z","last_seen_utc":"2026-01-02T00:00:00Z","last_failure_utc":"2026-01-02T00:00:00Z","active_sessions":0,"logins_ok":1,"logins_failed":3,"ip":"10.0.0.7","agent":"okhttp"}]}`

func TestConnections_ClientsTextJSONAndQuery(t *testing.T) {
	signIn_t(t)
	server, seen := answerServer(t, 200, clientsBody)
	code, out, _ := runBoth(t, "connections", "--server", server, "--kind", "studio", "--active", "--user", "ana")
	if code != ExitOK {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{"KIND", "studio", "ana", "0.9.1", "android", "c2"} {
		if !strings.Contains(out, want) {
			t.Errorf("text lacks %q:\n%s", want, out)
		}
	}
	got := seen()[0]
	if got.Method != "GET" || got.Path != "/api/connections/clients" || got.Query != "active=1&kind=studio&user=ana" || got.Authorization != "Bearer observe-test-token" {
		t.Fatalf("request: %+v", got)
	}
	object := jsonObject(t, "connections", "clients", "--server", server, "--json")
	requireResult(t, object, "connections clients")
	clients, _ := object["clients"].([]any)
	if len(clients) != 2 {
		t.Fatalf("clients: %v", object)
	}
	second, _ := clients[1].(map[string]any)
	if second["ip"] != "10.0.0.7" || second["id"] != "c2" || second["active_sessions"] != float64(0) {
		t.Fatalf("second client: %v", second)
	}
	if strings.Contains(out, "observe-test-token") {
		t.Fatal("the token was printed")
	}
}

func TestConnections_EventsTextJSONAndQuery(t *testing.T) {
	signIn_t(t)
	body := `{"events":[{"schema_version":"1.0","id":12,"at_utc":"2026-01-02T00:00:00Z","action":"login_failed","client_kind":"android","client_id":"c2","user":"bob","origin":"remote"}]}`
	server, seen := answerServer(t, 200, body)
	code, out, _ := runBoth(t, "connections", "events", "--server", server, "--action", "login_failed", "--limit", "5", "--before", "40")
	if code != ExitOK || !strings.Contains(out, "login_failed") || !strings.Contains(out, "bob") {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if got := seen()[0]; got.Path != "/api/connections/events" || got.Query != "action=login_failed&before=40&limit=5" {
		t.Fatalf("request: %+v", got)
	}
	object := jsonObject(t, "connections", "events", "--server", server, "--json")
	requireResult(t, object, "connections events")
	events, _ := object["events"].([]any)
	if len(events) != 1 || events[0].(map[string]any)["id"] != float64(12) {
		t.Fatalf("events: %v", object)
	}
}

func TestConnections_FiltersAreCheckedBeforeAnythingIsSent(t *testing.T) {
	signIn_t(t)
	server, seen := answerServer(t, 200, clientsBody)
	for _, args := range [][]string{
		{"connections", "--kind", "toaster"},
		{"connections", "events", "--action", "teleport"},
		{"connections", "events", "--limit", "abc"},
		{"connections", "events", "--limit", "1234567890"},
		{"connections", "events", "--before", "-1"},
		{"connections", "--action", "login"},
		{"connections", "events", "--active"},
		{"connections", "extra"},
	} {
		code, out, _ := runBoth(t, append(args, "--server", server)...)
		if code != ExitUsageError || out != "" {
			t.Errorf("%v: exit %d stdout %q", args, code, out)
		}
	}
	if n := len(seen()); n != 0 {
		t.Fatalf("%d request(s) were sent for bad arguments", n)
	}
}

// An answer that does not match the contract is an error and prints nothing.
func TestObserve_MalformedAnswersFailClosed(t *testing.T) {
	signIn_t(t)
	good := `{"schema_version":"1.0","id":"c1","kind":"studio","first_seen_utc":"a","last_seen_utc":"b","active_sessions":1,"logins_ok":1,"logins_failed":0}`
	cases := map[string][]string{
		"clients: not an object":      {"[]", "connections"},
		"clients: empty body":         {"", "connections"},
		"clients: trailing garbage":   {`{"clients":[]} x`, "connections"},
		"clients: null list":          {`{"clients":null}`, "connections"},
		"clients: no list":            {`{}`, "connections"},
		"clients: wrong schema":       {`{"clients":[` + strings.Replace(good, `"1.0"`, `"2.0"`, 1) + `]}`, "connections"},
		"clients: unknown kind":       {`{"clients":[` + strings.Replace(good, `"studio"`, `"toaster"`, 1) + `]}`, "connections"},
		"clients: no id":              {`{"clients":[` + strings.Replace(good, `"id":"c1",`, ``, 1) + `]}`, "connections"},
		"clients: missing count":      {`{"clients":[` + strings.Replace(good, `"logins_ok":1,`, ``, 1) + `]}`, "connections"},
		"clients: negative count":     {`{"clients":[` + strings.Replace(good, `"logins_ok":1`, `"logins_ok":-1`, 1) + `]}`, "connections"},
		"clients: count as text":      {`{"clients":[` + strings.Replace(good, `"logins_ok":1`, `"logins_ok":"1"`, 1) + `]}`, "connections"},
		"clients: bad origin":         {`{"clients":[` + strings.Replace(good, `"kind"`, `"origin":"moon","kind"`, 1) + `]}`, "connections"},
		"events: unknown action":      {`{"events":[{"schema_version":"1.0","id":1,"at_utc":"a","action":"teleport","client_kind":"studio","client_id":"c"}]}`, "connections events"},
		"events: no id":               {`{"events":[{"schema_version":"1.0","at_utc":"a","action":"login","client_kind":"studio","client_id":"c"}]}`, "connections events"},
		"liveness: bad state":         {`{"schema_version":"1.0","stale_after_s":30,"robots":[{"controller_id":"c","robot_id":1,"state":"asleep"}]}`, "liveness"},
		"liveness: unknown with news": {`{"schema_version":"1.0","stale_after_s":30,"robots":[{"controller_id":"c","robot_id":1,"state":"unknown","age_s":3,"last_update_utc":"a"}]}`, "liveness"},
		"liveness: live without news": {`{"schema_version":"1.0","stale_after_s":30,"robots":[{"controller_id":"c","robot_id":1,"state":"live"}]}`, "liveness"},
		"liveness: no limit":          {`{"schema_version":"1.0","robots":[]}`, "liveness"},
		"liveness: null robots":       {`{"schema_version":"1.0","stale_after_s":30,"robots":null}`, "liveness"},
		"ecosystem: summary lies":     {strings.Replace(ecosystemBody, `"total":2`, `"total":3`, 1), "ecosystem-health"},
		"ecosystem: unknown reason":   {strings.Replace(ecosystemBody, `"health_ok"`, `"vibes"`, 1), "ecosystem-health"},
		"ecosystem: unknown state":    {strings.Replace(ecosystemBody, `"state":"ok"`, `"state":"great"`, 1), "ecosystem-health"},
		"ecosystem: no drift":         {strings.Replace(ecosystemBody, `"drift":false,`, ``, 1), "ecosystem-health"},
		"ecosystem: wrong schema":     {strings.Replace(ecosystemBody, `"schema_version":"1.0"`, `"schema_version":"9.9"`, 1), "ecosystem-health"},
		"ecosystem: bad warning code": {strings.Replace(ecosystemBody, `"service_down"`, `"cosmic_rays"`, 1), "ecosystem-health"},
	}
	for name, c := range cases {
		server, _ := answerServer(t, 200, c[0])
		for _, mode := range [][]string{{}, {"--json"}} {
			args := append(append(strings.Fields(c[1]), "--server", server), mode...)
			code, out, errText := runBoth(t, args...)
			if code != ExitServerError || out != "" || !strings.Contains(errText, "unexpected response") {
				t.Errorf("%s %v: exit %d, stdout %q, stderr %q", name, mode, code, out, errText)
			}
		}
	}
}

func TestObserve_StatusCodesMapToExitCodes(t *testing.T) {
	signIn_t(t)
	for command, path := range map[string]string{"connections": "/api/connections/clients", "liveness": "/api/robots/liveness", "ecosystem-health": "/api/ecosystem/health"} {
		for status, want := range map[int]ExitCode{401: ExitAuthError, 403: ExitAuthError, 404: ExitServerError, 400: ExitRefused, 500: ExitServerError, 503: ExitServerError} {
			server, _ := answerServer(t, status, `{"error":"no"}`)
			code, out, errText := runBoth(t, command, "--server", server)
			if code != want || out != "" {
				t.Errorf("%s %d: exit %d (want %d), stdout %q, stderr %q", command, status, code, want, out, errText)
			}
			if status == 404 && (!strings.Contains(errText, path) || !strings.Contains(errText, "switched off")) {
				t.Errorf("%s 404: the message does not say what is missing: %q", command, errText)
			}
		}
	}
	// no credential at all
	clearCredentials(t)
	server, _ := answerServer(t, 401, `{"error":"no"}`)
	if code, _, errText := runBoth(t, "liveness", "--server", server); code != ExitAuthError || !strings.Contains(errText, "hydra-cli login") {
		t.Errorf("without a credential: exit %d, %q", code, errText)
	}
	// nothing listens
	if code, _, _ := runBoth(t, "liveness", "--server", "http://127.0.0.1:1"); code != ExitNetworkError && code != ExitAuthError {
		t.Errorf("unreachable: exit %d", code)
	}
}

// Text from the server never reaches the terminal as a control character, and nothing it did not declare is passed on.
func TestObserve_HostileTextAndUnknownFieldsAreNotPassedOn(t *testing.T) {
	signIn_t(t)
	body := `{"clients":[{"schema_version":"1.0","id":"c1","kind":"studio","user":"evil\u001b[2Jname\nFORGED","secret_extra":"do-not-pass","first_seen_utc":"a","last_seen_utc":"b","active_sessions":0,"logins_ok":0,"logins_failed":0,"agent":"Bearer abcdefghijklmnopqrstuvwxyz0123456789"}]}`
	server, _ := answerServer(t, 200, body)
	_, text, _ := runBoth(t, "connections", "--server", server)
	_, asJSON, _ := runBoth(t, "connections", "--server", server, "--json")
	for _, out := range []string{text, asJSON} {
		if strings.ContainsRune(out, 0x1b) || strings.Contains(out, "FORGED\n") || strings.Contains(out, "do-not-pass") || strings.Contains(out, "abcdefghijklmnopqrstuvwxyz0123456789") {
			t.Fatalf("hostile content got through:\n%q", out)
		}
	}
	if !strings.Contains(asJSON, "evil?[2J") {
		t.Fatalf("the control characters were not replaced:\n%s", asJSON)
	}
}

const livenessBody = `{"schema_version":"1.0","stale_after_s":30,"robots":[
 {"controller_id":"localhost","robot_id":1,"state":"live","last_update_utc":"2026-01-02T00:00:00Z","age_s":2.5,"source":"heartbeat"},
 {"controller_id":"localhost","robot_id":2,"state":"unknown"}]}`

func TestLiveness_TextAndJSON(t *testing.T) {
	signIn_t(t)
	server, seen := answerServer(t, 200, livenessBody)
	code, out, _ := runBoth(t, "liveness", "--server", server)
	if code != ExitOK {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{"CONTROLLER", "live", "2.5", "heartbeat", "unknown", "30 s"} {
		if !strings.Contains(out, want) {
			t.Errorf("text lacks %q:\n%s", want, out)
		}
	}
	if got := seen()[0]; got.Method != "GET" || got.Path != "/api/robots/liveness" || got.Body != "" {
		t.Fatalf("request: %+v", got)
	}
	object := jsonObject(t, "liveness", "--json", "--server", server)
	requireResult(t, object, "liveness")
	robots, _ := object["robots"].([]any)
	if object["stale_after_s"] != float64(30) || len(robots) != 2 || robots[0].(map[string]any)["age_s"] != 2.5 {
		t.Fatalf("liveness: %v", object)
	}
	if _, has := robots[1].(map[string]any)["last_update_utc"]; has {
		t.Fatalf("an unknown robot must not carry news: %v", robots[1])
	}
	empty, _ := answerServer(t, 200, `{"schema_version":"1.0","stale_after_s":30,"robots":[]}`)
	if code, out, _ := runBoth(t, "liveness", "--server", empty); code != ExitOK || !strings.Contains(out, "no robot") {
		t.Fatalf("empty roster: %d %q", code, out)
	}
}

const ecosystemBody = `{"schema_version":"1.0","generated_at_utc":"2026-01-02T00:00:00Z","available":true,
 "summary":{"total":2,"ok":1,"degraded":0,"down":1,"unknown":0,"drift":1},
 "services":[
  {"name":"HYDRA-UMC-SERVER","family":"hydra","role":"server","state":"ok","reason":"health_ok","version_expected":"0.9.1","version_reported":"0.9.0","drift":true,"last_seen_utc":"2026-01-02T00:00:00Z","latency_ms":12.5,"port":3000,"health_path":"/health","systemd_unit":"hydra.service","active_state":"active"},
  {"name":"HYDRA-UMC-DATALAKE","family":null,"role":null,"state":"down","reason":"unreachable","version_expected":null,"version_reported":null,"drift":false,"last_seen_utc":null,"latency_ms":null}],
 "certificate":{"state":"expiring","not_after_utc":"2026-02-01T00:00:00Z","days_left":12,"warn_days":30},
 "warnings":[{"code":"certificate_expiring"},{"code":"service_down","service":"HYDRA-UMC-DATALAKE"},{"code":"version_drift","service":"HYDRA-UMC-SERVER"}]}`

func TestEcosystemHealth_TextAndJSON(t *testing.T) {
	signIn_t(t)
	server, seen := answerServer(t, 200, ecosystemBody)
	code, out, _ := runBoth(t, "ecosystem-health", "--server", server)
	if code != ExitOK {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{"Services: 2 (ok 1, degraded 0, down 1, unknown 0); version drift: 1", "HYDRA-UMC-SERVER", "health_ok", "0.9.1", "unreachable", "Certificate: expiring", "12 days left", "Warning: service_down (HYDRA-UMC-DATALAKE)", "Warning: certificate_expiring"} {
		if !strings.Contains(out, want) {
			t.Errorf("text lacks %q:\n%s", want, out)
		}
	}
	if got := seen()[0]; got.Method != "GET" || got.Path != "/api/ecosystem/health" {
		t.Fatalf("request: %+v", got)
	}
	object := jsonObject(t, "ecosystem-health", "--json", "--server", server)
	requireResult(t, object, "ecosystem-health")
	services, _ := object["services"].([]any)
	if len(services) != 2 {
		t.Fatalf("services: %v", object)
	}
	first := services[0].(map[string]any)
	second := services[1].(map[string]any)
	if first["port"] != float64(3000) || first["drift"] != true {
		t.Fatalf("first: %v", first)
	}
	if v, has := second["family"]; !has || v != nil {
		t.Fatalf("a null field must stay null: %v", second)
	}
	if _, has := second["port"]; has {
		t.Fatalf("an operator view has no port: %v", second)
	}
	if object["certificate"].(map[string]any)["state"] != "expiring" {
		t.Fatalf("certificate: %v", object["certificate"])
	}
}

func TestEcosystemHealth_NoServicesAndNoCertificate(t *testing.T) {
	signIn_t(t)
	body := `{"schema_version":"1.0","generated_at_utc":"x","available":false,"summary":{"total":0,"ok":0,"degraded":0,"down":0,"unknown":0,"drift":0},"services":[],"certificate":null,"warnings":[]}`
	server, _ := answerServer(t, 200, body)
	code, out, _ := runBoth(t, "ecosystem-health", "--server", server)
	if code != ExitOK || !strings.Contains(out, "could not scan") || strings.Contains(out, "Certificate") {
		t.Fatalf("exit %d\n%s", code, out)
	}
	object := jsonObject(t, "ecosystem-health", "--json", "--server", server)
	if v, has := object["certificate"]; !has || v != nil {
		t.Fatalf("certificate must be null: %v", object)
	}
}

// ---- plan-simulate -----------------------------------------------------------

const planBody = `{"steps":[{"id":"a","primitive":"MOVE_TO","robot":"r1","params":{"location":"bin"}}],"constraints":{"robots":["r1"]}}`

const acceptedVerdict = `{"verdict":"accepted","executable":false,"executed":false,"violations":[],"timeline":[{"step_id":"a","index":0,"robot":"r1","primitive":"MOVE_TO","start_ms":0,"end_ms":1000}],"makespan_ms":1000,"simulated_clock_ms":1000}`

const rejectedVerdict = `{"verdict":"rejected","executable":false,"executed":false,"violations":[{"rule":"workspace_limit","step_index":0,"step_id":"a","message":"outside the box","value":[900.0,0.0,100.0],"limit":{"min":[0,0,0],"max":[500,500,300]},"suggestion":"clamp 'far' to [500.0, 0.0, 100.0]"}],"timeline":[],"makespan_ms":0,"simulated_clock_ms":0}`

func writePlan(t *testing.T, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPlanSimulate_AcceptedAndTheCredentialStaysHome(t *testing.T) {
	signIn_t(t)
	planner, seen := answerServer(t, 200, acceptedVerdict)
	var out bytes.Buffer
	if err := cmdPlanSimulate(&out, nil, []string{"--plan", writePlan(t, planBody), "--planner", planner}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Plan ACCEPTED by the simulation (nothing was executed)", "Makespan 1000 ms", "MOVE_TO", "r1"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("text lacks %q:\n%s", want, out.String())
		}
	}
	got := seen()[0]
	if got.Method != "POST" || got.Path != "/plan/simulate" || got.Body != planBody {
		t.Fatalf("request: %+v", got)
	}
	if got.Authorization != "" {
		t.Fatalf("the credential of the server was sent to the planner: %q", got.Authorization)
	}
	out.Reset()
	if err := cmdPlanSimulate(&out, nil, []string{"--plan", writePlan(t, planBody), "--planner", planner, "--json"}); err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(out.Bytes(), &object); err != nil {
		t.Fatal(err)
	}
	requireResult(t, object, "plan-simulate")
	if object["verdict"] != "accepted" || object["executed"] != false || object["makespan_ms"] != float64(1000) {
		t.Fatalf("verdict: %v", object)
	}
}

func TestPlanSimulate_RejectedPrintsEveryRuleAndExits8(t *testing.T) {
	signIn_t(t)
	planner, _ := answerServer(t, 200, rejectedVerdict)
	var out bytes.Buffer
	err := cmdPlanSimulate(&out, nil, []string{"--plan", writePlan(t, planBody), "--planner", planner})
	if exitCodeFor(err) != ExitRefused {
		t.Fatalf("want exit 8, got %v", err)
	}
	for _, want := range []string{"Plan REJECTED", "broken: workspace_limit at step a: outside the box", "suggestion: clamp"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("text lacks %q:\n%s", want, out.String())
		}
	}
	out.Reset()
	err = cmdPlanSimulate(&out, nil, []string{"--plan", writePlan(t, planBody), "--planner", planner, "--json"})
	if exitCodeFor(err) != ExitRefused {
		t.Fatalf("want exit 8 with --json too, got %v", err)
	}
	var object map[string]any
	if json.Unmarshal(out.Bytes(), &object) != nil || object["ok"] != false || object["verdict"] != "rejected" {
		t.Fatalf("json: %s", out.String())
	}
	violations, _ := object["violations"].([]any)
	first, _ := violations[0].(map[string]any)
	if first["rule"] != "workspace_limit" || first["limit"] == nil || first["value"] == nil {
		t.Fatalf("violation: %v", first)
	}
}

func TestPlanSimulate_ReadsThePlanFromStandardInputAndTheEnvironment(t *testing.T) {
	signIn_t(t)
	planner, seen := answerServer(t, 200, acceptedVerdict)
	t.Setenv(plannerEnv, planner)
	var out bytes.Buffer
	if err := cmdPlanSimulate(&out, strings.NewReader(planBody), []string{"--plan", "-"}); err != nil {
		t.Fatal(err)
	}
	if got := seen(); len(got) != 1 || got[0].Body != planBody {
		t.Fatalf("seen: %+v", got)
	}
}

func TestPlanSimulate_FailsClosedOnAnythingItDoesNotUnderstand(t *testing.T) {
	signIn_t(t)
	bad := map[string]string{
		"executed true":         strings.Replace(acceptedVerdict, `"executed":false`, `"executed":true`, 1),
		"executable true":       strings.Replace(acceptedVerdict, `"executable":false`, `"executable":true`, 1),
		"no executed":           strings.Replace(acceptedVerdict, `"executed":false,`, ``, 1),
		"no verdict":            strings.Replace(acceptedVerdict, `"verdict":"accepted",`, ``, 1),
		"other verdict":         strings.Replace(acceptedVerdict, `"accepted"`, `"probably"`, 1),
		"accepted with a rule":  strings.Replace(acceptedVerdict, `"violations":[]`, `"violations":[{"rule":"x","message":"m"}]`, 1),
		"rejected without rule": strings.Replace(rejectedVerdict, `"violations":[{"rule":"workspace_limit","step_index":0,"step_id":"a","message":"outside the box","value":[900.0,0.0,100.0],"limit":{"min":[0,0,0],"max":[500,500,300]},"suggestion":"clamp 'far' to [500.0, 0.0, 100.0]"}]`, `"violations":[]`, 1),
		"null timeline":         strings.Replace(acceptedVerdict, `"timeline":[{"step_id":"a","index":0,"robot":"r1","primitive":"MOVE_TO","start_ms":0,"end_ms":1000}]`, `"timeline":null`, 1),
		"time goes backwards":   strings.Replace(acceptedVerdict, `"end_ms":1000`, `"end_ms":-5`, 1),
		"no makespan":           strings.Replace(acceptedVerdict, `"makespan_ms":1000,`, ``, 1),
		"not an object":         `["accepted"]`,
		"empty":                 ``,
		"trailing":              acceptedVerdict + `{}`,
	}
	for name, body := range bad {
		planner, _ := answerServer(t, 200, body)
		var out bytes.Buffer
		err := cmdPlanSimulate(&out, nil, []string{"--plan", writePlan(t, planBody), "--planner", planner, "--json"})
		if exitCodeFor(err) != ExitServerError || out.Len() != 0 {
			t.Errorf("%s: err %v, printed %q", name, err, out.String())
		}
	}
}

func TestPlanSimulate_StatusCodesAndArguments(t *testing.T) {
	signIn_t(t)
	plan := writePlan(t, planBody)
	for status, want := range map[int]ExitCode{400: ExitUsageError, 404: ExitServerError, 500: ExitServerError, 401: ExitServerError} {
		planner, _ := answerServer(t, status, `{"error":"the reason"}`)
		err := cmdPlanSimulate(io.Discard, nil, []string{"--plan", plan, "--planner", planner})
		if exitCodeFor(err) != want {
			t.Errorf("status %d: %v", status, err)
		}
		if status == 404 && (err == nil || !strings.Contains(err.Error(), "plan_simulation.enabled")) {
			t.Errorf("404 must say how to switch it on: %v", err)
		}
		if status == 400 && (err == nil || !strings.Contains(err.Error(), "the reason")) {
			t.Errorf("400 must carry the reason: %v", err)
		}
	}
	planner, seen := answerServer(t, 200, acceptedVerdict)
	for _, args := range [][]string{
		{},
		{"--plan", plan, "--server", "http://localhost:3000"},
		{"--plan", plan, "--planner", "ftp://x"},
		{"--plan", plan, "--planner", planner, "extra"},
		{"--plan", writePlan(t, `["not","an","object"]`), "--planner", planner},
		{"--plan", writePlan(t, `not json`), "--planner", planner},
		{"--plan", filepath.Join(t.TempDir(), "missing.json"), "--planner", planner},
	} {
		if err := cmdPlanSimulate(io.Discard, nil, args); exitCodeFor(err) != ExitUsageError {
			t.Errorf("%v: %v", args, err)
		}
	}
	if n := len(seen()); n != 0 {
		t.Fatalf("%d request(s) were sent for bad arguments", n)
	}
	if err := cmdPlanSimulate(io.Discard, nil, []string{"--plan", plan, "--planner", "http://127.0.0.1:1"}); exitCodeFor(err) != ExitNetworkError {
		t.Errorf("unreachable planner: %v", err)
	}
}
