// =============================================================================
// HYDRA-UMC-TOOL-CLI - cmd/hydra-cli/edge_test.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0 - see LICENSE
// =============================================================================
package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// captureStdout runs fn and returns what it wrote to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = writer
	done := make(chan string)
	go func() {
		data, _ := io.ReadAll(reader)
		done <- string(data)
	}()
	func() {
		defer func() { os.Stdout = original }()
		fn()
	}()
	_ = writer.Close()
	return <-done
}

// captureStderr is captureStdout for os.Stderr.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = writer
	done := make(chan string)
	go func() {
		data, _ := io.ReadAll(reader)
		done <- string(data)
	}()
	func() {
		defer func() { os.Stderr = original }()
		fn()
	}()
	_ = writer.Close()
	return <-done
}

// linesWithPrefix counts the lines of text that begin with prefix.
func linesWithPrefix(text, prefix string) int {
	count := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, prefix) {
			count++
		}
	}
	return count
}

func hostileServer(t *testing.T, info, settings string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/hydra-info":
			_, _ = w.Write([]byte(info))
		case "/api/settings":
			_, _ = w.Write([]byte(settings))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// ----- text that comes from the server is never allowed to drive the terminal

func TestPrintable_ReplacesEveryControlCharacter(t *testing.T) {
	cases := map[string]string{
		"plain":                  "plain",
		"two\nlines":             "two?lines",
		"carriage\rreturn":       "carriage?return",
		"\x1b[2J clear":          "?[2J clear",
		"bell\a":                 "bell?",
		"tab\there":              "tab?here",
		"del\x7f":                "del?",
		"c1\u009bcsi":            "c1?csi",
		"unicode stays: ñ 日本語 ✓": "unicode stays: ñ 日本語 ✓",
		"":                       "",
	}
	for input, want := range cases {
		if got := printable(input); got != want {
			t.Errorf("printable(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestRobots_AHostileNameCannotForgeRowsOrEscapeCodes(t *testing.T) {
	clearCredentials(t)
	server := hostileServer(t, `{}`,
		`{"controllers":[{"name":"Mas\nter","robots":[{"name":"A\u001b[31m1\nFAKE-ROW yes","online":true,"model":"m\rx","role":"r\u0007"}]}]}`)
	defer server.Close()

	var out bytes.Buffer
	if err := cmdRobots(&out, []string{"--server", server.URL}); err != nil {
		t.Fatalf("cmdRobots: %v", err)
	}
	text := out.String()
	if strings.ContainsAny(strings.ReplaceAll(text, "\n", ""), "\x1b\r\a") {
		t.Fatalf("control characters reached the terminal: %q", text)
	}
	if lines := strings.Count(text, "\n"); lines != 2 {
		t.Fatalf("expected a header and exactly one row, got %d lines: %q", lines, text)
	}
}

func TestStatus_AHostileServerCannotInjectLinesOrEscapeCodes(t *testing.T) {
	server := hostileServer(t,
		`{"appVersion":"1.0\nstatus:      forged","status":"ok\u001b[0m","product":"P\rQ","hostname":"h\u0007"}`, `{}`)
	defer server.Close()

	var err error
	out := captureStdout(t, func() { err = cmdStatus([]string{"--server", server.URL}) })
	if err != nil {
		t.Fatalf("cmdStatus: %v", err)
	}
	if strings.ContainsAny(out, "\x1b\r\a") {
		t.Fatalf("control characters reached the terminal: %q", out)
	}
	if got := linesWithPrefix(out, "status:"); got != 1 {
		t.Fatalf("a forged status line got through (%d): %q", got, out)
	}
}

func TestDoctor_AHostileVersionCannotForgeAPassLine(t *testing.T) {
	server := hostileServer(t,
		`{"schema_version":"1.0","appVersion":"1\nDOCTOR=PASS forged","controllerCount":0,"robotCount":0}`,
		`{"controllers":[]}`)
	defer server.Close()

	var out bytes.Buffer
	if err := cmdDoctor(&out, []string{"--server", server.URL}); err != nil {
		t.Fatalf("cmdDoctor: %v", err)
	}
	if got := linesWithPrefix(out.String(), "DOCTOR=PASS"); got != 1 {
		t.Fatalf("a forged DOCTOR=PASS line got through (%d): %q", got, out.String())
	}
}

func TestAnErrorBodyFromTheServerIsPrintedWithoutControlCharacters(t *testing.T) {
	clearCredentials(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom\n\x1b[31mred"))
	}))
	defer server.Close()

	for name, call := range map[string]func() error{
		"status": func() error { return cmdStatus([]string{"--server", server.URL}) },
		"robots": func() error { return cmdRobots(&bytes.Buffer{}, []string{"--server", server.URL}) },
		"doctor": func() error { return cmdDoctor(&bytes.Buffer{}, []string{"--server", server.URL}) },
	} {
		err := call()
		if err == nil || exitCodeFor(err) != ExitServerError {
			t.Fatalf("%s: err = %v, want a server error", name, err)
		}
		if strings.ContainsAny(err.Error(), "\n\x1b") {
			t.Errorf("%s: the error text carries control characters: %q", name, err.Error())
		}
	}
}

// ----- config files

func TestValidateConfig_OnlyHTTPAndHTTPSServersAreValid(t *testing.T) {
	for _, bad := range []string{"ftp://host", "file:///etc/passwd", "ws://host:3000", "javascript://x", "mailto:a@b.c"} {
		if err := validateConfig(Config{Server: bad, TimeoutSec: 5}); err == nil {
			t.Errorf("validateConfig accepted %q although every live command would refuse it", bad)
		}
	}
	for _, good := range []string{"http://host", "https://host:8443", "http://127.0.0.1:3000/base"} {
		if err := validateConfig(Config{Server: good, TimeoutSec: 5}); err != nil {
			t.Errorf("validateConfig rejected %q: %v", good, err)
		}
	}
}

func TestLoadConfig_HostileFiles(t *testing.T) {
	for name, contents := range map[string]string{
		"empty file":     ``,
		"json array":     `[]`,
		"null":           `null`,
		"string timeout": `{"server":"http://h","timeoutSec":"5"}`,
		"float timeout":  `{"server":"http://h","timeoutSec":1.5}`,
		"huge timeout":   `{"server":"http://h","timeoutSec":99999999999999999999}`,
		"truncated":      `{"server":"http://h","timeoutSec":`,
		"numeric server": `{"server":5,"timeoutSec":5}`,
		"scheme only":    `{"server":"http://","timeoutSec":5}`,
		"binary":         "\x00\x01\x02",
		"utf8 bom":       "\xef\xbb\xbf{\"server\":\"http://h\",\"timeoutSec\":5}",
	} {
		_, err := loadConfig(writeConfigFile(t, contents))
		if err == nil {
			t.Errorf("%s: loadConfig accepted the file", name)
			continue
		}
		if got := exitCodeFor(err); got != ExitConfigError {
			t.Errorf("%s: exit code = %d, want ExitConfigError", name, got)
		}
	}
}

func TestLoadConfig_UnknownFieldsAreIgnored(t *testing.T) {
	cfg, err := loadConfig(writeConfigFile(t, `{"server":"http://h:1","timeoutSec":3,"future":{"a":1}}`))
	if err != nil || cfg.Server != "http://h:1" || cfg.TimeoutSec != 3 {
		t.Fatalf("cfg = %+v, err = %v", cfg, err)
	}
}

func TestParseConfigFlags_AnyOrderAndDanglingFlag(t *testing.T) {
	path, dry := parseConfigFlags([]string{"--dry-run", "--config", "a.json"})
	if path != "a.json" || !dry {
		t.Fatalf("got %q %v", path, dry)
	}
	path, dry = parseConfigFlags([]string{"--config"})
	if path != "" || dry {
		t.Fatalf("a dangling --config must give no path, got %q %v", path, dry)
	}
}

// ----- server argument validation

func TestNormalizeServer_Table(t *testing.T) {
	ok := map[string]string{
		"http://h":            "http://h",
		"  http://h:3000/  ":  "http://h:3000",
		"https://h//":         "https://h",
		"HTTP://H":            "HTTP://H",
		"http://[::1]:3000":   "http://[::1]:3000",
		"http://10.0.0.5/api": "http://10.0.0.5/api",
	}
	for in, want := range ok {
		got, err := normalizeServer(in)
		if err != nil || got != want {
			t.Errorf("normalizeServer(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "   ", "host:3000", "//host", "ftp://h", "http://", "http:/h", "http://h\x7f", "ws://h", "/just/a/path"} {
		if _, err := normalizeServer(bad); err == nil || exitCodeFor(err) != ExitUsageError {
			t.Errorf("normalizeServer(%q): err = %v, want a usage error", bad, err)
		}
	}
}

func TestParsePositiveSeconds_Table(t *testing.T) {
	for _, bad := range []string{"", "0", "-3", "abc", "1.5", " 5", "5s", "99999999999999999999"} {
		if _, ok := parsePositiveSeconds(bad); ok {
			t.Errorf("parsePositiveSeconds(%q) was accepted", bad)
		}
	}
	if d, ok := parsePositiveSeconds("7"); !ok || d.Seconds() != 7 {
		t.Errorf("parsePositiveSeconds(7) = %v, %v", d, ok)
	}
}

// ----- credentials

func TestRequireSecureCredentials_RefusesPlainHTTPAwayFromThisMachine(t *testing.T) {
	t.Setenv(allowInsecureEnv, "")
	activeTransport = transportSettings{}
	cases := map[string]bool{ // true: refused
		"http://10.0.0.5:3000":  true,
		"http://example.org":    true,
		"https://example.org":   false,
		"http://localhost:3000": false,
		"http://127.0.0.1:3000": false,
		"http://[::1]:3000":     false,
		"%zz":                   false,
	}
	for server, wantRefused := range cases {
		err := requireSecureCredentials(server)
		if got := err != nil; got != wantRefused {
			t.Errorf("requireSecureCredentials(%q) = %v, want refused = %v", server, err, wantRefused)
		}
		if err != nil && exitCodeFor(err) != ExitConfigError {
			t.Errorf("requireSecureCredentials(%q): exit code %d, want ExitConfigError", server, exitCodeFor(err))
		}
	}
}

func TestRequireSecureCredentials_AllowedByTheOperatorOnlyWarns(t *testing.T) {
	activeTransport = transportSettings{AllowInsecure: true}
	defer func() { activeTransport = transportSettings{} }()
	var err error
	out := captureStderr(t, func() { err = requireSecureCredentials("http://10.0.0.5:3000") })
	if err != nil || !strings.Contains(out, "warning") {
		t.Fatalf("allowed insecure: err = %v, stderr = %q, want a warning and no refusal", err, out)
	}
	quiet := captureStderr(t, func() { _ = requireSecureCredentials("http://127.0.0.1:3000") })
	if quiet != "" {
		t.Fatalf("this machine needs no warning, got %q", quiet)
	}
}

func TestResolveToken_LoginAnswersThatAreNotUsable(t *testing.T) {
	clearCredentials(t)
	t.Setenv(userEnv, "operator")
	t.Setenv(passwordEnv, "right-password")

	cases := map[string]struct {
		status int
		body   string
		want   ExitCode
	}{
		"empty token":      {http.StatusOK, `{"token":""}`, ExitServerError},
		"no token field":   {http.StatusOK, `{"success":true}`, ExitServerError},
		"not json":         {http.StatusOK, `<html>`, ExitServerError},
		"token not string": {http.StatusOK, `{"token":5}`, ExitServerError},
		"server error":     {http.StatusInternalServerError, `oops`, ExitServerError},
		"forbidden":        {http.StatusForbidden, `{}`, ExitAuthError},
		"rate limited":     {http.StatusTooManyRequests, `{}`, ExitServerError},
	}
	for name, c := range cases {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(c.status)
			_, _ = w.Write([]byte(c.body))
		}))
		err := cmdRobots(&bytes.Buffer{}, []string{"--server", server.URL})
		server.Close()
		if err == nil || exitCodeFor(err) != c.want {
			t.Errorf("%s: err = %v (code %d), want code %d", name, err, exitCodeFor(err), c.want)
		}
	}
}

func TestResolveToken_ATokenWithOnlySpacesIsNotACredential(t *testing.T) {
	clearCredentials(t)
	t.Setenv(tokenEnv, "   ")
	var sawAuth bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization") != ""
		_, _ = w.Write([]byte(`{"controllers":[]}`))
	}))
	defer server.Close()
	if err := cmdRobots(&bytes.Buffer{}, []string{"--server", server.URL}); err != nil {
		t.Fatalf("cmdRobots: %v", err)
	}
	if sawAuth {
		t.Fatal("a blank token was sent as a bearer credential")
	}
}

func TestResolveToken_TheTokenIsTrimmedBeforeItIsSent(t *testing.T) {
	clearCredentials(t)
	t.Setenv(tokenEnv, "  abc  \n")
	var header string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"controllers":[]}`))
	}))
	defer server.Close()
	if err := cmdRobots(&bytes.Buffer{}, []string{"--server", server.URL}); err != nil {
		t.Fatalf("cmdRobots: %v", err)
	}
	if header != "Bearer abc" {
		t.Fatalf("Authorization = %q, want \"Bearer abc\"", header)
	}
}

func TestReadBody_TheLimitIsExact(t *testing.T) {
	exact := strings.NewReader(strings.Repeat("x", maxResponseBytes))
	if body, err := readBody(exact); err != nil || len(body) != maxResponseBytes {
		t.Fatalf("a body of exactly the cap: len=%d err=%v", len(body), err)
	}
	over := strings.NewReader(strings.Repeat("x", maxResponseBytes+1))
	if _, err := readBody(over); err != errResponseTooLarge {
		t.Fatalf("one byte over the cap: err = %v", err)
	}
}

// ----- the dispatcher

func TestRun_EveryCommandIsRoutedWithItsOwnExitCode(t *testing.T) {
	clearCredentials(t)
	server := hostileServer(t,
		`{"schema_version":"1.0","appVersion":"1.0","controllerCount":1,"robotCount":1}`,
		`{"controllers":[{"name":"M","robots":[{"name":"R"}]}]}`)
	defer server.Close()
	cfg := writeConfigFile(t, `{"server":"http://127.0.0.1:1","timeoutSec":1}`)
	badCfg := writeConfigFile(t, `{`)

	cases := []struct {
		args []string
		want ExitCode
	}{
		{[]string{"-v"}, ExitOK},
		{[]string{"--version"}, ExitOK},
		{[]string{"help"}, ExitOK},
		{[]string{"-h"}, ExitOK},
		{[]string{"--help"}, ExitOK},
		{[]string{"status", "--server", server.URL}, ExitOK},
		{[]string{"robots", "--server", server.URL}, ExitOK},
		{[]string{"doctor", "--server", server.URL}, ExitOK},
		{[]string{"doctor", "--server", server.URL, "--json"}, ExitOK},
		{[]string{"completion", "bash"}, ExitOK},
		{[]string{"config", "validate", "--config", cfg}, ExitOK},
		{[]string{"config", "apply", "--config", cfg, "--dry-run"}, ExitOK},
		{[]string{"config", "apply", "--config", cfg}, ExitNotImplemented},
		{[]string{"config", "validate", "--config", badCfg}, ExitConfigError},
		{[]string{"config"}, ExitUsageError},
		{[]string{"status", "--bogus"}, ExitUsageError},
		{[]string{"status", "--server"}, ExitUsageError},
		{[]string{"status", "--server", "ftp://x"}, ExitUsageError},
		{[]string{"robots", "--config", badCfg}, ExitConfigError},
		{[]string{"doctor", "--server", "http://127.0.0.1:1"}, ExitNetworkError},
		{[]string{"completion", "tcsh"}, ExitUsageError},
		{[]string{"completion"}, ExitUsageError},
		{[]string{"Status"}, ExitUsageError},
		{[]string{""}, ExitUsageError},
		{[]string{"--bogus"}, ExitUsageError},
	}
	for _, c := range cases {
		var got ExitCode
		_ = captureStdout(t, func() {
			_ = captureStderr(t, func() { got = run(c.args) })
		})
		if got != c.want {
			t.Errorf("run(%q) = %d, want %d", c.args, got, c.want)
		}
	}
}

func TestRun_ShellReadsCommandsFromStdinAndReturnsOK(t *testing.T) {
	clearCredentials(t)
	original := os.Stdin
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdin = reader
	defer func() { os.Stdin = original }()
	_, _ = writer.WriteString("version\nexit\n")
	_ = writer.Close()

	var got ExitCode
	out := captureStdout(t, func() { got = run([]string{"shell"}) })
	if got != ExitOK || !strings.Contains(out, projectName) {
		t.Fatalf("run(shell) = %d, output %q", got, out)
	}
}

func TestRun_ShellRejectsStrayArguments(t *testing.T) {
	var got ExitCode
	_ = captureStderr(t, func() { got = run([]string{"shell", "--bogus"}) })
	if got != ExitUsageError {
		t.Fatalf("run(shell --bogus) = %d, want ExitUsageError", got)
	}
}
