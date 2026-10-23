// =============================================================================
// HYDRA-UMC-TOOL-CLI - cmd/hydra-cli/record_test.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0 - see LICENSE
// =============================================================================
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// ifMatchServer answers by path and records the If-Match header of every request.
func ifMatchServer(t *testing.T, handler func(r *http.Request, rec recorded) (int, string)) (string, *[]string) {
	t.Helper()
	var mu sync.Mutex
	headers := []string{}
	srv, _ := fakeServer(t, func(r *http.Request, rec recorded) (int, string) {
		mu.Lock()
		headers = append(headers, rec.Method+" "+rec.Path+" if-match="+r.Header.Get("If-Match"))
		mu.Unlock()
		return handler(r, rec)
	})
	return srv.URL, &headers
}

const limitsAnswer = `{"limits":{"jog":{"default":{"maxAmount":100,"maxAbsolute":1000}}},"robot":{"id":3,"model":"Parol6 (6-DOF)","jog":{"maxAmount":100,"maxAbsolute":300}}}`

func TestIfMatchIsSentQuotedByClaimReleaseAndJog(t *testing.T) {
	server, headers := ifMatchServer(t, func(r *http.Request, rec recorded) (int, string) {
		if strings.HasPrefix(r.URL.Path, "/api/safety-limits") {
			return 200, limitsAnswer
		}
		return 200, `{"success":true,"revision":"after-write"}`
	})
	var out bytes.Buffer
	if err := cmdClaim(&out, []string{"--server", server, "3", "--if-match", "rev-1"}); err != nil {
		t.Fatal(err)
	}
	if err := cmdRelease(&out, []string{"--server", server, "3", "--if-match", "rev-2"}); err != nil {
		t.Fatal(err)
	}
	if err := cmdJog(&out, []string{"--server", server, "3", "x", "5", "--if-match", "rev-3"}); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(*headers, "\n")
	for _, want := range []string{`POST /api/robot/3/claim if-match="rev-1"`, `POST /api/robot/3/release if-match="rev-2"`, `POST /api/robot/3/command if-match="rev-3"`} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in\n%s", want, joined)
		}
	}
	if strings.Count(out.String(), "Revision: after-write") != 3 {
		t.Fatalf("each write should print the revision it left the robot at:\n%s", out.String())
	}
}

func TestNoIfMatchByDefaultAndNoneForStop(t *testing.T) {
	server, headers := ifMatchServer(t, func(r *http.Request, rec recorded) (int, string) { return 200, `{"success":true}` })
	if err := cmdClaim(io.Discard, []string{"--server", server, "3"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(*headers, "\n"), `if-match="`) {
		t.Fatalf("a write without --if-match must not carry one: %v", *headers)
	}
}

func TestIfMatchCurrentReadsTheRobotFirst(t *testing.T) {
	server, headers := ifMatchServer(t, func(r *http.Request, rec recorded) (int, string) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/robot/3" {
			return 200, `{"robot":{"id":3,"name":"Arm","model":"Parol6"},"revision":"now-rev","cellId":"c1"}`
		}
		return 200, `{"success":true,"revision":"later"}`
	})
	if err := cmdRelease(io.Discard, []string{"--server", server, "3", "--if-match", "current"}); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(*headers, "\n")
	if !strings.Contains(joined, "GET /api/robot/3 if-match=") || !strings.Contains(joined, `POST /api/robot/3/release if-match="now-rev"`) {
		t.Fatalf("expected a read and then a release with the revision read:\n%s", joined)
	}
}

func TestIfMatchCurrentWithoutARevisionSendsNothing(t *testing.T) {
	server, headers := ifMatchServer(t, func(r *http.Request, rec recorded) (int, string) {
		return 200, `{"robot":{"id":3}}`
	})
	err := cmdRelease(io.Discard, []string{"--server", server, "3", "--if-match", "current"})
	if err == nil || exitCodeFor(err) != ExitServerError {
		t.Fatalf("want a server error, got %v", err)
	}
	for _, h := range *headers {
		if strings.HasPrefix(h, "POST") {
			t.Fatalf("nothing may be sent without a revision: %v", *headers)
		}
	}
}

func TestIfMatchValueIsValidated(t *testing.T) {
	for _, bad := range []string{"has space", "semi;colon", strings.Repeat("a", 129), `"quoted"`} {
		err := cmdClaim(io.Discard, []string{"--server", "http://127.0.0.1:1", "3", "--if-match", bad})
		if err == nil || exitCodeFor(err) != ExitUsageError {
			t.Fatalf("%q: want a usage error, got %v", bad, err)
		}
	}
}

func TestRevisionMismatchIsRefusedWithTheCurrentRevision(t *testing.T) {
	server, _ := ifMatchServer(t, func(r *http.Request, rec recorded) (int, string) {
		return 412, `{"error":"the robot changed since you read it","code":"revision-mismatch","revision":"current-rev"}`
	})
	err := cmdRelease(io.Discard, []string{"--server", server, "3", "--if-match", "stale"})
	if err == nil || exitCodeFor(err) != ExitRefused {
		t.Fatalf("want a refusal, got %v", err)
	}
	for _, want := range []string{"changed since the revision you gave", "current-rev", "nothing was applied", "hydra-cli robot"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("message lacks %q: %v", want, err)
		}
	}
}

func TestRobotPrintsTheRevision(t *testing.T) {
	server, _ := ifMatchServer(t, func(r *http.Request, rec recorded) (int, string) {
		return 200, `{"robot":{"id":3,"name":"Arm","model":"Parol6 (6-DOF)","role":"Idle"},"revision":"abc123","cellId":"cell-1"}`
	})
	var out bytes.Buffer
	if err := cmdRobot(&out, []string{"--server", server, "3"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Robot 3: Arm (Parol6 (6-DOF))", "Revision: abc123", "Cell:     cell-1"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output lacks %q:\n%s", want, out.String())
		}
	}
	out.Reset()
	if err := cmdRobot(&out, []string{"--server", server, "3", "--json"}); err != nil || !strings.Contains(out.String(), `"revision": "abc123"`) {
		t.Fatalf("json output: %v\n%s", err, out.String())
	}
}

func TestRobotOnAnOlderServerSaysSo(t *testing.T) {
	server, _ := ifMatchServer(t, func(r *http.Request, rec recorded) (int, string) {
		return 200, `{"robot":{"id":3,"name":"Arm","model":"Z1"}}`
	})
	var out bytes.Buffer
	if err := cmdRobot(&out, []string{"--server", server, "3"}); err != nil || !strings.Contains(out.String(), "not reported") {
		t.Fatalf("%v\n%s", err, out.String())
	}
	missing, _ := ifMatchServer(t, func(r *http.Request, rec recorded) (int, string) { return 404, `{"error":"Robot not found"}` })
	if err := cmdRobot(io.Discard, []string{"--server", missing, "3"}); err == nil {
		t.Fatal("a robot the server does not know must fail")
	}
	if err := cmdRobot(io.Discard, []string{"--server", server}); exitCodeFor(err) != ExitUsageError {
		t.Fatalf("usage: %v", err)
	}
}

func TestTableSendsOneAtomicRequest(t *testing.T) {
	var sent map[string]any
	server, headers := ifMatchServer(t, func(r *http.Request, rec recorded) (int, string) {
		if strings.HasPrefix(r.URL.Path, "/api/safety-limits") {
			return 200, limitsAnswer
		}
		sent = rec.Body
		return 200, `{"success":true,"revision":"t1","applied":{"robotId":3,"x":50,"y":20,"rz":15}}`
	})
	var out bytes.Buffer
	if err := cmdTable(&out, []string{"--server", server, "--controller-id", "cli-1", "3", "50", "20", "--rz", "15", "--if-match", "rev-9"}); err != nil {
		t.Fatal(err)
	}
	params, _ := sent["params"].(map[string]any)
	if sent["command"] != "table" || params["x"] != float64(50) || params["y"] != float64(20) || params["rz"] != float64(15) || params["mode"] != "absolute" || sent["controllerId"] != "cli-1" {
		t.Fatalf("unexpected request: %v", sent)
	}
	if !strings.Contains(strings.Join(*headers, "\n"), `POST /api/robot/3/command if-match="rev-9"`) {
		t.Fatalf("headers: %v", *headers)
	}
	for _, want := range []string{"Table of robot 3 is now at x 50, y 20, rz 15.", "Revision: t1"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output lacks %q:\n%s", want, out.String())
		}
	}
}

func TestTableRelativeAndLimits(t *testing.T) {
	var sent map[string]any
	posts := 0
	server, _ := ifMatchServer(t, func(r *http.Request, rec recorded) (int, string) {
		if strings.HasPrefix(r.URL.Path, "/api/safety-limits") {
			return 200, limitsAnswer // relative up to 100, absolute up to 300 for this model
		}
		posts++
		sent = rec.Body
		return 200, `{"success":true,"applied":{"x":1,"y":2}}`
	})
	if err := cmdTable(io.Discard, []string{"--server", server, "3", "5", "-5", "--relative"}); err != nil {
		t.Fatal(err)
	}
	if params, _ := sent["params"].(map[string]any); params["mode"] != "relative" {
		t.Fatalf("mode: %v", sent)
	}
	for _, args := range [][]string{{"3", "101", "0", "--relative"}, {"3", "301", "0"}, {"3", "0", "-301"}, {"3", "0", "0", "--rz", "400"}} {
		err := cmdTable(io.Discard, append([]string{"--server", server}, args...))
		if err == nil || exitCodeFor(err) != ExitRefused || !strings.Contains(err.Error(), "nothing was sent") {
			t.Fatalf("%v: want a refusal before sending, got %v", args, err)
		}
	}
	if posts != 1 {
		t.Fatalf("only the first move may have been sent, %d were", posts)
	}
}

func TestTableUsageAndOlderServer(t *testing.T) {
	server, _ := ifMatchServer(t, func(r *http.Request, rec recorded) (int, string) {
		if strings.HasPrefix(r.URL.Path, "/api/safety-limits") {
			return 200, limitsAnswer
		}
		return 400, `{"error":"unknown command"}`
	})
	for _, args := range [][]string{{"3", "1"}, {"3", "a", "2"}, {"3", "NaN", "2"}, {"x", "1", "2"}, {"3", "1", "2", "--rz", "q"}} {
		if err := cmdTable(io.Discard, append([]string{"--server", server}, args...)); exitCodeFor(err) != ExitUsageError {
			t.Fatalf("%v: want a usage error, got %v", args, err)
		}
	}
	err := cmdTable(io.Discard, []string{"--server", server, "3", "1", "2"})
	if err == nil || exitCodeFor(err) != ExitRefused || !strings.Contains(err.Error(), "does not have the table command") {
		t.Fatalf("an older server: %v", err)
	}
}

func TestTableRefusalsOfTheServer(t *testing.T) {
	server, _ := ifMatchServer(t, func(r *http.Request, rec recorded) (int, string) {
		if strings.HasPrefix(r.URL.Path, "/api/safety-limits") {
			return 200, limitsAnswer
		}
		return 400, `{"error":"the table command requires a configured XY table","code":"table-missing"}`
	})
	err := cmdTable(io.Discard, []string{"--server", server, "3", "1", "2"})
	if err == nil || exitCodeFor(err) != ExitRefused || !strings.Contains(err.Error(), "XY table") {
		t.Fatalf("got %v", err)
	}
}

// ---- audit -------------------------------------------------------------------

const genesis = "0000000000000000000000000000000000000000000000000000000000000000"

// auditChain writes lines the way the server does: compact JSON, the hash of the line without `hash`, which is added last.
func auditChain(count int, start int) []string {
	lines := []string{}
	prev := genesis
	for i := 0; i < count; i++ {
		body := fmt.Sprintf(`{"seq":%d,"ts":"2030-01-01T00:00:0%d.000Z","type":"claim","outcome":"ok","username":"ana","controllerId":"w1","ip":"10.0.0.5","robotId":1,"detail":{"ttlMs":60000},"prev":"%s"}`, start+i, i, prev)
		sum := sha256.Sum256([]byte(body))
		digest := hex.EncodeToString(sum[:])
		lines = append(lines, body[:len(body)-1]+`,"hash":"`+digest+`"}`)
		prev = digest
	}
	return lines
}

func auditBody(lines []string, extra string) string {
	return `{"enabled":true,"lockedOff":false,"entries":[` + strings.Join(lines, ",") + `]` + extra + `}`
}

func auditServer(t *testing.T, status int, body string) (string, *[]recorded) {
	srv, seen := fakeServer(t, func(r *http.Request, rec recorded) (int, string) { return status, body })
	return srv.URL, seen
}

func TestAuditChainIntact(t *testing.T) {
	server, seen := auditServer(t, 200, auditBody(auditChain(4, 1), `,"verification":{"ok":true,"entries":4}`))
	var out bytes.Buffer
	if err := cmdAudit(&out, []string{"--server", server, "--limit", "50", "--type", "claim", "--robot", "1", "--verify"}); err != nil {
		t.Fatal(err)
	}
	if got := (*seen)[0]; got.Path != "/api/audit" || !strings.Contains(got.Query, "limit=50") || !strings.Contains(got.Query, "type=claim") || !strings.Contains(got.Query, "robotId=1") || !strings.Contains(got.Query, "verify=1") {
		t.Fatalf("unexpected request: %+v", got)
	}
	for _, want := range []string{"Server: the whole chain is intact (4 lines).", "4 lines read, every hash matches, 3 links between neighbours checked, 0 gaps", "ana", "ttlMs=60000"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output lacks %q:\n%s", want, out.String())
		}
	}
	// the newest line first
	if strings.Index(out.String(), "\n4 ") > strings.Index(out.String(), "\n1 ") {
		t.Fatalf("the newest line must come first:\n%s", out.String())
	}
}

func TestAuditChangedLineIsFoundAndExitsRefused(t *testing.T) {
	lines := auditChain(4, 1)
	lines[2] = strings.Replace(lines[2], `"username":"ana"`, `"username":"mallory"`, 1)
	server, _ := auditServer(t, 200, auditBody(lines, ""))
	var out bytes.Buffer
	err := cmdAudit(&out, []string{"--server", server})
	if err == nil || exitCodeFor(err) != ExitRefused {
		t.Fatalf("want a refusal, got %v", err)
	}
	if !strings.Contains(out.String(), "line 3 does not match its own hash") {
		t.Fatalf("output:\n%s", out.String())
	}
}

func TestAuditRemovedLineIsFound(t *testing.T) {
	lines := auditChain(5, 1)
	// the fourth line renumbered to follow the second: its hash no longer matches what was written
	server, _ := auditServer(t, 200, auditBody([]string{lines[0], lines[1], lines[3], lines[4]}, ""))
	var out bytes.Buffer
	if err := cmdAudit(&out, []string{"--server", server}); err != nil {
		t.Fatalf("a window with a hole is only a gap, not a broken chain: %v", err)
	}
	if !strings.Contains(out.String(), "1 gaps not comparable") {
		t.Fatalf("output:\n%s", out.String())
	}
}

func TestAuditReplacedLineWhoseSuccessorStillNamesTheOldOne(t *testing.T) {
	lines := auditChain(3, 1)
	body := lines[1][:strings.Index(lines[1], `,"hash":"`)] + "}"
	body = strings.Replace(body, `"username":"ana"`, `"username":"someone-else"`, 1)
	sum := sha256.Sum256([]byte(body))
	rewritten := body[:len(body)-1] + `,"hash":"` + hex.EncodeToString(sum[:]) + `"}`
	server, _ := auditServer(t, 200, auditBody([]string{lines[0], rewritten, lines[2]}, ""))
	var out bytes.Buffer
	err := cmdAudit(&out, []string{"--server", server})
	if err == nil || exitCodeFor(err) != ExitRefused || !strings.Contains(out.String(), "line 3 does not name the line before it") {
		t.Fatalf("%v\n%s", err, out.String())
	}
}

func TestAuditServerSaysBroken(t *testing.T) {
	server, _ := auditServer(t, 200, auditBody(auditChain(2, 1), `,"verification":{"ok":false,"entries":2,"brokenAtSeq":2,"reason":"a line was changed"}`))
	var out bytes.Buffer
	err := cmdAudit(&out, []string{"--server", server, "--verify"})
	if err == nil || exitCodeFor(err) != ExitRefused || !strings.Contains(out.String(), "Server: the chain is BROKEN at line 2 (a line was changed).") {
		t.Fatalf("%v\n%s", err, out.String())
	}
}

func TestAuditOffRecordAndEmpty(t *testing.T) {
	server, _ := auditServer(t, 200, `{"enabled":false,"lockedOff":true,"entries":[]}`)
	var out bytes.Buffer
	if err := cmdAudit(&out, []string{"--server", server}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"locked off by the environment", "No lines for this filter."} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output lacks %q:\n%s", want, out.String())
		}
	}
}

func TestAuditRefusals(t *testing.T) {
	cases := []struct {
		status int
		want   string
		code   ExitCode
	}{
		{403, "not an administrator", ExitAuthError},
		{404, "no audit record", ExitServerError},
		{401, "token expired", ExitAuthError},
	}
	for _, c := range cases {
		server, _ := auditServer(t, c.status, `{"error":"x"}`)
		err := cmdAudit(io.Discard, []string{"--server", server})
		if err == nil || exitCodeFor(err) != c.code || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("status %d: %v", c.status, err)
		}
	}
}

func TestAuditExportSavesTheLinesAsStored(t *testing.T) {
	stored := strings.Join(auditChain(2, 1), "\n") + "\n"
	srv, seen := fakeServer(t, func(r *http.Request, rec recorded) (int, string) { return 200, stored })
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	var out bytes.Buffer
	if err := cmdAudit(&out, []string{"--server", srv.URL, "--export", path, "--limit", "10"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains((*seen)[0].Query, "format=jsonl") || !strings.Contains((*seen)[0].Query, "limit=10") {
		t.Fatalf("query: %s", (*seen)[0].Query)
	}
	written, err := os.ReadFile(path)
	if err != nil || string(written) != stored {
		t.Fatalf("written: %v %q", err, written)
	}
	if err := cmdAudit(io.Discard, []string{"--server", srv.URL, "--export", filepath.Join(t.TempDir(), "no", "such", "dir", "a.jsonl")}); exitCodeFor(err) != ExitGeneralError {
		t.Fatalf("an unwritable path: %v", err)
	}
}

func TestAuditJSONOutputCarriesTheChainCheck(t *testing.T) {
	server, _ := auditServer(t, 200, auditBody(auditChain(3, 5), ""))
	var out bytes.Buffer
	if err := cmdAudit(&out, []string{"--server", server, "--json"}); err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Entries []json.RawMessage `json:"entries"`
		Chain   struct {
			OK      bool `json:"ok"`
			Checked int  `json:"checked"`
			Links   int  `json:"links"`
		} `json:"chain"`
	}
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil || len(parsed.Entries) != 3 || !parsed.Chain.OK || parsed.Chain.Links != 2 {
		t.Fatalf("%v %+v", err, parsed)
	}
}

func TestAuditArgumentsAreValidated(t *testing.T) {
	for _, args := range [][]string{{"--limit", "0"}, {"--limit", "1001"}, {"--limit", "x"}, {"--robot", "x"}, {"extra"}} {
		if err := cmdAudit(io.Discard, append([]string{"--server", "http://127.0.0.1:1"}, args...)); exitCodeFor(err) != ExitUsageError {
			t.Fatalf("%v: want a usage error, got %v", args, err)
		}
	}
}

func TestCheckAuditChainDirectly(t *testing.T) {
	var raw []json.RawMessage
	for _, line := range auditChain(5, 1) {
		raw = append(raw, json.RawMessage(line))
	}
	lines := parseAuditLines(raw)
	if got := checkAuditChain(lines); !got.OK || got.Checked != 5 || got.Links != 4 || got.Gaps != 0 {
		t.Fatalf("%+v", got)
	}
	if got := checkAuditChain([]auditLine{lines[0], lines[2], lines[4]}); !got.OK || got.Links != 0 || got.Gaps != 2 {
		t.Fatalf("a filtered view: %+v", got)
	}
	// a line pretty-printed on the way cannot be re-read byte for byte: it is counted, not called broken
	pretty := json.RawMessage(strings.ReplaceAll(auditChain(1, 1)[0], `","`, "\",\n\""))
	got := checkAuditChain(parseAuditLines([]json.RawMessage{pretty}))
	if !got.OK || got.Unreadable != 1 {
		t.Fatalf("%+v", got)
	}
}
