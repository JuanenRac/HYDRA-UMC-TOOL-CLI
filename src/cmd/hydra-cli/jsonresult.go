// =============================================================================
// HYDRA-UMC-TOOL-CLI - The JSON result of the commands that act: cmd/hydra-cli/jsonresult.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0-or-later - see LICENSE
// =============================================================================
// Every command that prints data also prints it as JSON with --json. The commands that read keep the shape
// they had (an array or the answer of the server, documented in docs/CLI_REFERENCE.md); the commands that act
// (stop, claim, release, jog-arm, jog, table, login, logout, config, version) and the commands added with the
// JSON mode print one object with a stable shape:
//
//	{"schema_version": "1.0", "command": "<name>", "ok": true, ...fields of the command}
//
// The keys are sorted and the fields of one command never change within a schema_version, so a script can rely
// on them. The text of every command is the default and does not change. A failure is not printed as JSON: it
// goes to the standard error as text and sets the exit code, as it always did. Nothing here ever carries a
// token, a refresh token or a password.
package main

import (
	"encoding/json"
	"fmt"
	"io"
)

// jsonSchemaVersion is the version of the shape of the objects printed by writeJSONResult.
const jsonSchemaVersion = "1.0"

// writeJSONResult prints one result object. Text that came from a server goes through printable first.
func writeJSONResult(w io.Writer, command string, ok bool, fields map[string]any) error {
	out := map[string]any{"schema_version": jsonSchemaVersion, "command": command, "ok": ok}
	for key, value := range fields {
		if text, isText := value.(string); isText {
			value = printable(text)
		}
		out[key] = value
	}
	encoded, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return newCliError(ExitGeneralError, fmt.Errorf("could not encode the result: %w", err))
	}
	_, err = fmt.Fprintln(w, string(encoded))
	return err
}

// revisionOrNil is the revision an answer carries, or nil (JSON null) when it carries none.
func revisionOrNil(r controlResponse) any {
	if rev := r.revision(); rev != "" {
		return rev
	}
	return nil
}
