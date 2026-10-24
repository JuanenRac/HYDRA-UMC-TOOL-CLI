// =============================================================================
// HYDRA-UMC-TOOL-CLI - cmd/hydra-cli/main_test.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0-or-later - see LICENSE
// =============================================================================
package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestMain keeps the tests of this package from touching the real session file of whoever runs them, and from
// waiting a second every time a doctor test shows a count mismatch.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "hydra-cli-test-*")
	if err != nil {
		panic(err)
	}
	os.Setenv(tokenFileEnv, filepath.Join(dir, "session.json"))
	doctorRecheckDelay = time.Millisecond
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
