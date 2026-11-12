// =============================================================================
// HYDRA-UMC-TOOL-CLI - cmd/hydra-cli/flashall_test.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0-or-later - see LICENSE
// =============================================================================
// flash-all against a stand-in server that answers like POST /api/hardware/canota/flash of HYDRA-UMC-SERVER (it checks the same shape of
// the query and answers {success, finalPhase}); no board is involved.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
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

type flashCall struct {
	Query       map[string]string
	ContentType string
	Auth        string
	Body        []byte
}

type fakeFlashServer struct {
	*httptest.Server
	mu     sync.Mutex
	calls  []flashCall
	status map[string]int    // slot -> HTTP status (default 200)
	phase  map[string]string // slot -> final phase (default "done")
}

func newFakeFlashServer(t *testing.T) *fakeFlashServer {
	t.Helper()
	f := &fakeFlashServer{status: map[string]int{}, phase: map[string]string{}}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/hardware/canota/flash" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		query := map[string]string{}
		for key, values := range r.URL.Query() {
			query[key] = values[0]
		}
		f.mu.Lock()
		f.calls = append(f.calls, flashCall{Query: query, ContentType: r.Header.Get("Content-Type"), Auth: r.Header.Get("Authorization"), Body: body})
		slot := query["slot"]
		status, phase := f.status[slot], f.phase[slot]
		f.mu.Unlock()
		if status == 0 {
			status = 200
		}
		if phase == "" {
			phase = "done"
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if status >= 400 {
			_, _ = w.Write([]byte(`{"error":"stand-in refusal"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": phase == "done", "finalPhase": phase})
	}))
	t.Cleanup(f.Close)
	return f
}

func (f *fakeFlashServer) recorded() []flashCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]flashCall(nil), f.calls...)
}

func (f *fakeFlashServer) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = nil
}

func (f *fakeFlashServer) setStatus(slot string, status int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if status == 0 {
		f.status = map[string]int{}
		return
	}
	f.status[slot] = status
}

func (f *fakeFlashServer) setPhase(slot, phase string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.phase[slot] = phase
}

func (f *fakeFlashServer) slots() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, c := range f.calls {
		out = append(out, c.Query["slot"])
	}
	return out
}

func writeImage(t *testing.T, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "image.bin")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func flashArgs(server, image string, extra ...string) []string {
	args := []string{"--server", server, "--image", image, "--tier", "1", "--hardware-id", "7", "--image-version", "0.8.1"}
	return append(args, extra...)
}

var testSignature = strings.Repeat("ab", 64)

func TestFlashAll_DryRunSendsNothingAndNeedsNoLogin(t *testing.T) {
	clearCredentials(t)
	srv := newFakeFlashServer(t)
	image := writeImage(t, []byte("firmware bytes"))
	var out bytes.Buffer
	if err := cmdFlashAll(&out, flashArgs(srv.URL, image, "--dry-run", "--slots", "0-2")); err != nil {
		t.Fatalf("%v", err)
	}
	if len(srv.recorded()) != 0 {
		t.Fatal("a dry run reached the server")
	}
	sum := sha256.Sum256([]byte("firmware bytes"))
	text := out.String()
	for _, want := range []string{"Dry run: nothing was sent", hex.EncodeToString(sum[:]), "planned", "slot=0", "slot=1", "slot=2", "tier=1", "hardware_id=7", "version_major=0", "version_minor=8", "version_patch=1"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in\n%s", want, text)
		}
	}
	if strings.Contains(text, "slot=3") {
		t.Error("a slot that was not asked for is in the plan")
	}
}

func TestFlashAll_NeedsYesOrDryRunAndNeverBoth(t *testing.T) {
	clearCredentials(t)
	t.Setenv(tokenEnv, "t")
	srv := newFakeFlashServer(t)
	image := writeImage(t, []byte("x"))
	if err := cmdFlashAll(&bytes.Buffer{}, flashArgs(srv.URL, image)); exitCodeFor(err) != ExitUsageError || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("%v", err)
	}
	if err := cmdFlashAll(&bytes.Buffer{}, flashArgs(srv.URL, image, "--yes", "--dry-run")); exitCodeFor(err) != ExitUsageError {
		t.Fatalf("%v", err)
	}
	if len(srv.recorded()) != 0 {
		t.Fatal("something was sent without a confirmation")
	}
}

func TestFlashAll_SendsTheImageToEverySlotInOrderWithTheSignedQuery(t *testing.T) {
	clearCredentials(t)
	t.Setenv(tokenEnv, "tok")
	srv := newFakeFlashServer(t)
	content := []byte("signed firmware image")
	image := writeImage(t, content)
	var out bytes.Buffer
	args := flashArgs(srv.URL, image, "--yes", "--signature", strings.ToUpper(testSignature), "--target", "G474-ctrl", "--min-bootloader", "0.8.0", "--downgrade-from", "0.9.0", "--relay", "1")
	if err := cmdFlashAll(&out, args); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	if got := strings.Join(srv.slots(), ","); got != "0,1,2,3,4,5,6,7" {
		t.Fatalf("slots %s", got)
	}
	first := srv.recorded()[0]
	if first.ContentType != "application/octet-stream" || first.Auth != "Bearer tok" || !bytes.Equal(first.Body, content) {
		t.Fatalf("%+v", first)
	}
	want := map[string]string{"tier": "1", "hardware_id": "7", "relay": "1", "version_major": "0", "version_minor": "8", "version_patch": "1",
		"signature": testSignature, "target": "G474-ctrl", "min_bootloader": "0.8.0", "downgrade_from": "0.9.0"}
	for key, value := range want {
		if first.Query[key] != value {
			t.Errorf("%s = %q, want %q", key, first.Query[key], value)
		}
	}
	if !strings.Contains(out.String(), "8 of 8 slot(s) flashed.") {
		t.Fatalf("%s", out.String())
	}
}

func TestFlashAll_StopsAtTheFirstFailureUnlessKeepGoing(t *testing.T) {
	clearCredentials(t)
	t.Setenv(tokenEnv, "t")
	srv := newFakeFlashServer(t)
	srv.setPhase("2", "verify_failed")
	image := writeImage(t, []byte("x"))
	var out bytes.Buffer
	err := cmdFlashAll(&out, flashArgs(srv.URL, image, "--yes", "--slots", "0-4"))
	if exitCodeFor(err) != ExitServerError {
		t.Fatalf("%v", err)
	}
	if got := strings.Join(srv.slots(), ","); got != "0,1,2" {
		t.Fatalf("it went on after a failure: %s", got)
	}
	for _, want := range []string{"verify_failed", "not-tried", "2 of 5 slot(s) flashed."} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in\n%s", want, out.String())
		}
	}
	srv.reset()
	out.Reset()
	err = cmdFlashAll(&out, flashArgs(srv.URL, image, "--yes", "--slots", "0-4", "--keep-going"))
	if exitCodeFor(err) != ExitServerError || strings.Join(srv.slots(), ",") != "0,1,2,3,4" || !strings.Contains(out.String(), "4 of 5 slot(s) flashed.") {
		t.Fatalf("%v %v\n%s", err, srv.slots(), out.String())
	}
}

func TestFlashAll_AServerRefusalIsReportedAndStops(t *testing.T) {
	clearCredentials(t)
	t.Setenv(tokenEnv, "t")
	srv := newFakeFlashServer(t)
	srv.setStatus("0", 403)
	image := writeImage(t, []byte("x"))
	var out bytes.Buffer
	err := cmdFlashAll(&out, flashArgs(srv.URL, image, "--yes", "--slots", "0,1"))
	if exitCodeFor(err) != ExitAuthError || len(srv.recorded()) != 1 {
		t.Fatalf("%v calls=%d", err, len(srv.recorded()))
	}
	srv.setStatus("0", 0)
	srv.setStatus("0", 503)
	srv.reset()
	if err := cmdFlashAll(&bytes.Buffer{}, flashArgs(srv.URL, image, "--yes", "--slots", "0")); exitCodeFor(err) != ExitServerError {
		t.Fatalf("%v", err)
	}
}

func TestFlashAll_JSONHasAStableShapeAndNoCredential(t *testing.T) {
	clearCredentials(t)
	t.Setenv(tokenEnv, "a-very-secret-token-value")
	srv := newFakeFlashServer(t)
	image := writeImage(t, []byte("abc"))
	var out bytes.Buffer
	if err := cmdFlashAll(&out, flashArgs(srv.URL, image, "--yes", "--slots", "0,3", "--json")); err != nil {
		t.Fatal(err)
	}
	var doc struct {
		SchemaVersion string `json:"schema_version"`
		Command       string `json:"command"`
		OK            bool   `json:"ok"`
		DryRun        bool   `json:"dry_run"`
		ImageBytes    int    `json:"image_bytes"`
		Slots         []struct {
			Slot   int    `json:"slot"`
			Status string `json:"status"`
			Phase  string `json:"phase"`
		} `json:"slots"`
	}
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	if doc.SchemaVersion != "1.0" || doc.Command != "flash-all" || !doc.OK || doc.DryRun || doc.ImageBytes != 3 || len(doc.Slots) != 2 || doc.Slots[1].Slot != 3 || doc.Slots[1].Status != "flashed" || doc.Slots[1].Phase != "done" {
		t.Fatalf("%+v", doc)
	}
	if strings.Contains(out.String(), "a-very-secret-token-value") {
		t.Fatal("the credential is in the output")
	}
}

func TestFlashAll_TheOptionsAreCheckedBeforeAnythingIsSent(t *testing.T) {
	clearCredentials(t)
	t.Setenv(tokenEnv, "t")
	srv := newFakeFlashServer(t)
	image := writeImage(t, []byte("x"))
	empty := writeImage(t, nil)
	sigFile := filepath.Join(t.TempDir(), "sig")
	_ = os.WriteFile(sigFile, []byte(testSignature+"\nsecond line\n"), 0o600)
	badSig := filepath.Join(t.TempDir(), "badsig")
	_ = os.WriteFile(badSig, []byte("not hex\n"), 0o600)
	cases := map[string]struct {
		args []string
		code ExitCode
	}{
		"no image":             {[]string{"--server", srv.URL, "--yes", "--tier", "1", "--hardware-id", "1", "--image-version", "1.0.0"}, ExitUsageError},
		"missing image file":   {flashArgs(srv.URL, filepath.Join(t.TempDir(), "nope"), "--yes"), ExitConfigError},
		"empty image":          {flashArgs(srv.URL, empty, "--yes"), ExitConfigError},
		"slot 8":               {flashArgs(srv.URL, image, "--yes", "--slots", "8"), ExitUsageError},
		"reversed range":       {flashArgs(srv.URL, image, "--yes", "--slots", "5-2"), ExitUsageError},
		"slot twice":           {flashArgs(srv.URL, image, "--yes", "--slots", "1,1-2"), ExitUsageError},
		"slot text":            {flashArgs(srv.URL, image, "--yes", "--slots", "a"), ExitUsageError},
		"no tier":              {[]string{"--server", srv.URL, "--yes", "--image", image, "--hardware-id", "1", "--image-version", "1.0.0"}, ExitUsageError},
		"tier too big":         {flashArgs(srv.URL, image, "--yes", "--tier", "70000"), ExitUsageError},
		"no hardware id":       {[]string{"--server", srv.URL, "--yes", "--image", image, "--tier", "1", "--image-version", "1.0.0"}, ExitUsageError},
		"bad relay":            {flashArgs(srv.URL, image, "--yes", "--relay", "2"), ExitUsageError},
		"bad version":          {[]string{"--server", srv.URL, "--yes", "--image", image, "--tier", "1", "--hardware-id", "1", "--image-version", "1.0"}, ExitUsageError},
		"short signature":      {flashArgs(srv.URL, image, "--yes", "--signature", "abcd"), ExitUsageError},
		"both signatures":      {flashArgs(srv.URL, image, "--yes", "--signature", testSignature, "--signature-file", sigFile), ExitUsageError},
		"bad signature file":   {flashArgs(srv.URL, image, "--yes", "--signature-file", badSig), ExitUsageError},
		"missing sig file":     {flashArgs(srv.URL, image, "--yes", "--signature-file", filepath.Join(t.TempDir(), "nope")), ExitConfigError},
		"bad downgrade":        {flashArgs(srv.URL, image, "--yes", "--downgrade-from", "x"), ExitUsageError},
		"bad min bootloader":   {flashArgs(srv.URL, image, "--yes", "--min-bootloader", "1.2"), ExitUsageError},
		"bad target":           {flashArgs(srv.URL, image, "--yes", "--target", "a b&c"), ExitUsageError},
		"extra argument":       {flashArgs(srv.URL, image, "--yes", "now"), ExitUsageError},
		"option without value": {flashArgs(srv.URL, image, "--yes", "--slots"), ExitUsageError},
	}
	for name, c := range cases {
		if err := cmdFlashAll(&bytes.Buffer{}, c.args); exitCodeFor(err) != c.code {
			t.Errorf("%s: %v (exit %d, want %d)", name, err, exitCodeFor(err), c.code)
		}
	}
	if len(srv.recorded()) != 0 {
		t.Fatalf("something was sent although an option was wrong: %d calls", len(srv.recorded()))
	}
	// and the file of the signature is read from its first line
	if err := cmdFlashAll(&bytes.Buffer{}, flashArgs(srv.URL, image, "--yes", "--slots", "0", "--signature-file", sigFile)); err != nil {
		t.Fatal(err)
	}
	if srv.recorded()[0].Query["signature"] != testSignature {
		t.Fatalf("%v", srv.recorded()[0].Query)
	}
}

func TestFlashAll_ParseSlotList(t *testing.T) {
	good := map[string]string{"0-7": "0,1,2,3,4,5,6,7", "3": "3", "0,2,5": "0,2,5", "1-3,6": "1,2,3,6", "7,0": "7,0", " 1 , 2 ": "1,2"}
	for text, want := range good {
		slots, err := parseSlotList(text)
		if err != nil {
			t.Errorf("%q: %v", text, err)
			continue
		}
		var parts []string
		for _, s := range slots {
			parts = append(parts, string(rune('0'+s)))
		}
		if strings.Join(parts, ",") != want {
			t.Errorf("%q = %v", text, slots)
		}
	}
	for _, text := range []string{"", "8", "0-8", "-1", "1-", "a-b", "1;2", "00", "10"} {
		if _, err := parseSlotList(text); err == nil {
			t.Errorf("%q was accepted", text)
		}
	}
}

func TestFlashAll_TheHelpIsReadFromTheUsageAndTheServerIsNotNeeded(t *testing.T) {
	var code ExitCode
	out := captureStdout(t, func() { code = run([]string{"flash-all", "--help"}) })
	if code != ExitOK || !strings.HasPrefix(out, "USAGE: hydra-cli flash-all ") || !strings.Contains(out, "--dry-run") {
		t.Fatalf("exit %d %q", code, out)
	}
}
