// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveAppDirUsesEnvironmentOverride(t *testing.T) {
	custom := filepath.Join(t.TempDir(), "custom")
	t.Setenv(EnvAppDir, custom)

	if got := ResolveAppDir(); got != custom {
		t.Fatalf("expected %q, got %q", custom, got)
	}
}

func TestResolveAppDirFallsBackToUserData(t *testing.T) {
	if writableDir(SystemDir) {
		t.Skipf("%s es escribible en este entorno; se omite el fallback", SystemDir)
	}
	xdg := t.TempDir()
	t.Setenv(EnvAppDir, "")
	t.Setenv("XDG_DATA_HOME", xdg)

	want := filepath.Join(xdg, "httpaas")
	if got := ResolveAppDir(); got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestEnsureLayoutCreatesDirectories(t *testing.T) {
	base := filepath.Join(t.TempDir(), "httpaas")
	layout, err := EnsureLayout(base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if layout.ISODir != filepath.Join(base, "iso") {
		t.Errorf("unexpected ISO dir: %q", layout.ISODir)
	}
	if layout.InstancesFile != filepath.Join(base, "instances.json") {
		t.Errorf("unexpected instances file: %q", layout.InstancesFile)
	}
	if layout.SetupStateFile != filepath.Join(base, "setup-estado.json") {
		t.Errorf("unexpected setup state file: %q", layout.SetupStateFile)
	}
	for _, dir := range []string{layout.Base, layout.ISODir} {
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			t.Errorf("expected directory %s: %v", dir, err)
		}
	}
}

func TestEnsureLayoutRejectsEmptyBase(t *testing.T) {
	if _, err := EnsureLayout("  "); err == nil {
		t.Fatal("expected an error for an empty base directory")
	}
}
