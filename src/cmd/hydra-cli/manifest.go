// =============================================================================
// HYDRA-UMC-TOOL-CLI - `manifest validate`: cmd/hydra-cli/manifest.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0-or-later - see LICENSE
// =============================================================================
// Every repository of the ecosystem describes itself in hydra-umc.project.json, and the SDK publishes the
// schema of that file. `hydra-cli manifest validate` checks manifests against the schema, so a manifest
// that drifted (a missing field, a version in the wrong shape, a name that is not the repository's) is found
// before the dashboard, the updater or the server trips over it.
//
//	hydra-cli manifest validate [PATH...] [--workspace DIR] [--schema FILE] [--json]
//
// PATH is a manifest file or a repository directory. --workspace DIR checks the manifest of every
// HYDRA-UMC-* and URTC-* directory in DIR. With neither, the current directory is checked. The schema is the
// one of the SDK, embedded at build time (a test compares it with the SDK when the sibling checkout is
// there); --schema reads another file.
package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed schemas/project-manifest.schema.json
var embeddedManifestSchema []byte

const manifestFileName = "hydra-umc.project.json"

// manifestResult is the verdict for one manifest.
type manifestResult struct {
	Path       string            `json:"path"`
	OK         bool              `json:"ok"`
	Name       string            `json:"name,omitempty"`
	Version    string            `json:"version,omitempty"`
	Violations []schemaViolation `json:"violations,omitempty"`
	Error      string            `json:"error,omitempty"`
}

// manifestTargets expands PATHs and --workspace into the manifest files to check.
func manifestTargets(paths []string, workspace string) ([]string, error) {
	var files []string
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, newCliError(ExitConfigError, fmt.Errorf("%s: %w", path, err))
		}
		if info.IsDir() {
			path = filepath.Join(path, manifestFileName)
		}
		files = append(files, path)
	}
	if workspace != "" {
		entries, err := os.ReadDir(workspace)
		if err != nil {
			return nil, newCliError(ExitConfigError, fmt.Errorf("reading the workspace %s: %w", workspace, err))
		}
		for _, entry := range entries {
			if !entry.IsDir() || !(strings.HasPrefix(entry.Name(), "HYDRA-UMC") || strings.HasPrefix(entry.Name(), "URTC")) {
				continue
			}
			candidate := filepath.Join(workspace, entry.Name(), manifestFileName)
			if _, err := os.Stat(candidate); err == nil {
				files = append(files, candidate)
			}
		}
	}
	if len(files) == 0 && len(paths) == 0 && workspace == "" {
		files = append(files, manifestFileName)
	}
	sort.Strings(files)
	return files, nil
}

// checkManifest validates one file against the schema and against its own directory name.
func checkManifest(schema map[string]any, path string) manifestResult {
	result := manifestResult{Path: path}
	raw, err := os.ReadFile(path)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	document, err := decodeJSON(raw)
	if err != nil {
		result.Error = "not valid JSON: " + err.Error()
		return result
	}
	violations, err := validateAgainstSchema(schema, document)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	if object, ok := document.(map[string]any); ok {
		result.Name, _ = object["name"].(string)
		result.Version, _ = object["version"].(string)
		// a manifest says its repository's own name: a copy of another repository's manifest is wrong even if it fits the schema
		if abs, absErr := filepath.Abs(path); absErr == nil && result.Name != "" {
			directory := filepath.Base(filepath.Dir(abs))
			if (strings.HasPrefix(directory, "HYDRA-UMC") || strings.HasPrefix(directory, "URTC")) && directory != result.Name {
				violations = append(violations, schemaViolation{Path: "$.name", Message: fmt.Sprintf("is %q but the repository directory is %q", result.Name, directory)})
			}
		}
	}
	result.Violations = violations
	result.OK = len(violations) == 0
	return result
}

// cmdManifest implements `hydra-cli manifest validate`.
func cmdManifest(w io.Writer, args []string) error {
	if len(args) == 0 || args[0] != "validate" {
		return newCliError(ExitUsageError, errors.New("usage: hydra-cli manifest validate [PATH...] [--workspace DIR] [--schema FILE] [--json]"))
	}
	rest := args[1:]
	workspace, rest, err := takeFlag(rest, "--workspace")
	if err != nil {
		return err
	}
	schemaPath, rest, err := takeFlag(rest, "--schema")
	if err != nil {
		return err
	}
	jsonOutput, rest := takeSwitch(rest, "--json")
	for _, extra := range rest {
		if strings.HasPrefix(extra, "--") {
			return newCliError(ExitUsageError, fmt.Errorf("manifest validate does not know the option %s", extra))
		}
	}
	schemaBytes := embeddedManifestSchema
	if schemaPath != "" {
		schemaBytes, err = os.ReadFile(schemaPath)
		if err != nil {
			return newCliError(ExitConfigError, fmt.Errorf("reading the schema: %w", err))
		}
	}
	var schema map[string]any
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		return newCliError(ExitConfigError, fmt.Errorf("the schema is not valid JSON: %w", err))
	}
	files, err := manifestTargets(rest, workspace)
	if err != nil {
		return err
	}
	results := make([]manifestResult, 0, len(files))
	failed := 0
	for _, file := range files {
		result := checkManifest(schema, file)
		if !result.OK {
			failed++
		}
		results = append(results, result)
	}
	if jsonOutput {
		encoded, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return newCliError(ExitGeneralError, err)
		}
		fmt.Fprintln(w, string(encoded))
	} else {
		for _, result := range results {
			switch {
			case result.OK:
				fmt.Fprintf(w, "OK    %s (%s %s)\n", result.Path, printable(result.Name), printable(result.Version))
			case result.Error != "":
				fmt.Fprintf(w, "FAIL  %s: %s\n", result.Path, printable(result.Error))
			default:
				fmt.Fprintf(w, "FAIL  %s\n", result.Path)
				for _, violation := range result.Violations {
					fmt.Fprintf(w, "        %s\n", printable(violation.String()))
				}
			}
		}
		if failed == 0 {
			fmt.Fprintf(w, "MANIFESTS=PASS checked=%d\n", len(results))
		} else {
			fmt.Fprintf(w, "MANIFESTS=FAIL checked=%d failed=%d\n", len(results), failed)
		}
	}
	if failed > 0 {
		return newCliError(ExitConfigError, fmt.Errorf("%d of %d manifest(s) do not fit the schema", failed, len(results)))
	}
	return nil
}
