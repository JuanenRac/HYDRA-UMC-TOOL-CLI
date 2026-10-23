// HYDRA-UMC-TOOL-CLI - src/cmd/hydra-sign/main_test.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0-or-later - see LICENSE
//
// Signing and verification with keys made for the test; no real key is read.
package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fixture struct {
	dir     string
	private string
	public  string
	binary  string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	pubDer, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	f := fixture{dir: dir, private: filepath.Join(dir, "test-private.pem"), public: filepath.Join(dir, "test-public.pem"), binary: filepath.Join(dir, "hydra-cli")}
	if err := os.WriteFile(f.private, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.public, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDer}), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.binary, []byte("pretend this is a release binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	return f
}

func call(args ...string) (int, string, string) {
	var out, errOut bytes.Buffer
	code := run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestSignThenVerify(t *testing.T) {
	f := newFixture(t)
	if code, out, errOut := call("sign", "--key", f.private, f.binary); code != 0 || !strings.Contains(out, "signed hydra-cli") {
		t.Fatalf("sign: %d %q %q", code, out, errOut)
	}
	for _, suffix := range []string{".sha256", ".sig"} {
		if _, err := os.Stat(f.binary + suffix); err != nil {
			t.Fatalf("missing %s: %v", suffix, err)
		}
	}
	sha, _ := os.ReadFile(f.binary + ".sha256")
	if !strings.HasSuffix(strings.TrimSpace(string(sha)), "  hydra-cli") || len(strings.Fields(string(sha))[0]) != 64 {
		t.Fatalf("the .sha256 is not in the sha256sum format: %q", sha)
	}
	if code, out, errOut := call("verify", "--pub", f.public, f.binary); code != 0 || !strings.Contains(out, "OK hydra-cli") {
		t.Fatalf("verify: %d %q %q", code, out, errOut)
	}
}

func TestAChangedBinaryDoesNotVerify(t *testing.T) {
	f := newFixture(t)
	call("sign", "--key", f.private, f.binary)
	if err := os.WriteFile(f.binary, []byte("tampered"), 0o755); err != nil {
		t.Fatal(err)
	}
	code, _, errOut := call("verify", "--pub", f.public, f.binary)
	if code != 1 || !strings.Contains(errOut, ".sha256") {
		t.Fatalf("expected a digest failure, got %d %q", code, errOut)
	}
}

func TestAChangedSignatureOrAnotherKeyDoesNotVerify(t *testing.T) {
	f := newFixture(t)
	call("sign", "--key", f.private, f.binary)
	other := newFixture(t)
	if code, _, errOut := call("verify", "--pub", other.public, f.binary); code != 1 || !strings.Contains(errOut, "does not verify") {
		t.Fatalf("another key must fail: %d %q", code, errOut)
	}
	if err := os.WriteFile(f.binary+".sig", []byte("not base64 !!\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := call("verify", "--pub", f.public, f.binary); code != 1 {
		t.Fatal("a damaged signature must fail")
	}
}

func TestMissingSidecarFilesFail(t *testing.T) {
	f := newFixture(t)
	if code, _, errOut := call("verify", "--pub", f.public, f.binary); code != 1 || !strings.Contains(errOut, "missing") {
		t.Fatalf("unsigned file: %d %q", code, errOut)
	}
}

func TestASecondFileFailingStillReportsTheFirst(t *testing.T) {
	f := newFixture(t)
	second := filepath.Join(f.dir, "second")
	if err := os.WriteFile(second, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	call("sign", "--key", f.private, f.binary)
	code, out, _ := call("verify", "--pub", f.public, f.binary, second)
	if code != 1 || !strings.Contains(out, "OK hydra-cli") {
		t.Fatalf("expected OK for the first and a failure overall: %d %q", code, out)
	}
}

func TestThePrivateKeyIsNeverPrinted(t *testing.T) {
	f := newFixture(t)
	privatePEM, _ := os.ReadFile(f.private)
	lines := strings.Split(strings.TrimSpace(string(privatePEM)), "\n")
	body := strings.TrimSpace(lines[1])
	for _, args := range [][]string{{"sign", "--key", f.private, f.binary}, {"pubkey", "--key", f.private}, {"sign", "--key", f.binary, f.binary}} {
		_, out, errOut := call(args...)
		if strings.Contains(out+errOut, body) || strings.Contains(out+errOut, "BEGIN PRIVATE KEY") {
			t.Fatalf("%v printed key material", args)
		}
	}
}

func TestPubkeyPrintsTheMatchingPublicKey(t *testing.T) {
	f := newFixture(t)
	code, out, _ := call("pubkey", "--key", f.private)
	want, _ := os.ReadFile(f.public)
	if code != 0 || out != string(want) {
		t.Fatalf("pubkey: %d %q", code, out)
	}
}

func TestBadInputIsRefused(t *testing.T) {
	f := newFixture(t)
	notPEM := filepath.Join(f.dir, "junk.pem")
	_ = os.WriteFile(notPEM, []byte("junk"), 0o600)
	cases := [][]string{
		{},
		{"sign"},
		{"sign", "--key", f.private},
		{"sign", "--key", notPEM, f.binary},
		{"sign", "--key", filepath.Join(f.dir, "missing.pem"), f.binary},
		{"verify", "--pub", notPEM, f.binary},
		{"verify", "--pub", f.public},
		{"unknown"},
		{"pubkey"},
	}
	for _, args := range cases {
		if code, _, _ := call(args...); code == 0 {
			t.Errorf("%v should fail", args)
		}
	}
}
