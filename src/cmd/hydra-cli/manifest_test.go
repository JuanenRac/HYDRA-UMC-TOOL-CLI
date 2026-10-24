// =============================================================================
// HYDRA-UMC-TOOL-CLI - cmd/hydra-cli/manifest_test.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0-or-later - see LICENSE
// =============================================================================
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const goodManifest = `{"schema_version":"1.0","ecosystem":"HYDRA-UMC","name":"HYDRA-UMC-DEMO","version":"0.1.2","role":"cli","stack":"Go","technologies":["Go"],"deployment_target":"user-pc","maturity":"functional","family":"tools","parent":null,"native_version":{"file":"version.go","pattern":"x"},"build":"go build","notes":"n"}`

func repoWith(t *testing.T, dirName, manifest string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), dirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, manifestFileName), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestManifestValidate_AGoodManifestPasses(t *testing.T) {
	dir := repoWith(t, "HYDRA-UMC-DEMO", goodManifest)
	var out bytes.Buffer
	if err := cmdManifest(&out, []string{"validate", dir}); err != nil || !strings.Contains(out.String(), "MANIFESTS=PASS checked=1") {
		t.Fatalf("%v %q", err, out.String())
	}
}

func TestManifestValidate_EachKindOfDriftIsNamed(t *testing.T) {
	cases := map[string]string{
		"version":      strings.Replace(goodManifest, `"0.1.2"`, `"v1"`, 1),
		"maturity":     strings.Replace(goodManifest, `"functional"`, `"great"`, 1),
		"notes":        strings.Replace(goodManifest, `,"notes":"n"`, ``, 1),
		"extra":        strings.Replace(goodManifest, `"build":"go build"`, `"build":"go build","surprise":1`, 1),
		"technologies": strings.Replace(goodManifest, `["Go"]`, `["Go","Go"]`, 1),
	}
	for name, manifest := range cases {
		var out bytes.Buffer
		err := cmdManifest(&out, []string{"validate", repoWith(t, "HYDRA-UMC-DEMO", manifest)})
		if exitCodeFor(err) != ExitConfigError || !strings.Contains(out.String(), "MANIFESTS=FAIL") {
			t.Errorf("%s: %v %q", name, err, out.String())
		}
	}
}

func TestManifestValidate_ANameThatIsNotTheDirectoryFails(t *testing.T) {
	var out bytes.Buffer
	err := cmdManifest(&out, []string{"validate", repoWith(t, "HYDRA-UMC-OTHER", goodManifest)})
	if err == nil || !strings.Contains(out.String(), "repository directory") {
		t.Fatalf("%v %q", err, out.String())
	}
}

func TestManifestValidate_WorkspaceAndNotJSON(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{"HYDRA-UMC-DEMO": goodManifest, "URTC-X": `{`} {
		dir := filepath.Join(root, name)
		_ = os.MkdirAll(dir, 0o755)
		_ = os.WriteFile(filepath.Join(dir, manifestFileName), []byte(content), 0o644)
	}
	_ = os.MkdirAll(filepath.Join(root, "unrelated"), 0o755)
	var out bytes.Buffer
	err := cmdManifest(&out, []string{"validate", "--workspace", root})
	if err == nil || !strings.Contains(out.String(), "checked=2 failed=1") || !strings.Contains(out.String(), "not valid JSON") {
		t.Fatalf("%v %q", err, out.String())
	}
}

func TestManifestValidate_UsageAndForeignKeywords(t *testing.T) {
	if err := cmdManifest(&bytes.Buffer{}, nil); exitCodeFor(err) != ExitUsageError {
		t.Fatal(err)
	}
	schema := filepath.Join(t.TempDir(), "s.json")
	_ = os.WriteFile(schema, []byte(`{"type":"object","if":{}}`), 0o644)
	var out bytes.Buffer
	err := cmdManifest(&out, []string{"validate", repoWith(t, "HYDRA-UMC-DEMO", goodManifest), "--schema", schema})
	if err == nil || !strings.Contains(out.String(), "does not understand") {
		t.Fatalf("a keyword the checker does not read must not pass silently: %v %q", err, out.String())
	}
}

func TestEmbeddedSchemaMatchesTheSDKWhenItIsNextToThisRepository(t *testing.T) {
	sdk := filepath.Join("..", "..", "..", "..", "HYDRA-UMC-SDK", "contracts", "json-schema", "v1", "project-manifest.schema.json")
	want, err := os.ReadFile(sdk)
	if err != nil {
		t.Skip("the SDK is not checked out next to this repository")
	}
	norm := func(b []byte) string { return strings.ReplaceAll(string(b), "\r\n", "\n") }
	if norm(want) != norm(embeddedManifestSchema) {
		t.Fatal("schemas/project-manifest.schema.json differs from the SDK: copy the SDK file over it")
	}
}
