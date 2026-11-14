// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package provisioner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/ports"
)

// buildSeedISO genera una ISO NoCloud (volumen "cidata") con los archivos
// indicados, usando la primera herramienta disponible en la plataforma.
func buildSeedISO(ctx context.Context, executor ports.Executor, workDir, output string, files []string) error {
	if len(files) == 0 {
		return fmt.Errorf("no hay archivos para la ISO semilla")
	}
	_ = os.Remove(output)

	// Linux / BSD: genisoimage o mkisofs con Joliet y Rock Ridge.
	for _, tool := range []string{"genisoimage", "mkisofs"} {
		if _, err := exec.LookPath(tool); err != nil {
			continue
		}
		args := append([]string{
			"-quiet", "-output", output,
			"-volid", "cidata", "-joliet", "-rock",
		}, files...)
		return runISOTool(ctx, executor, tool, args...)
	}

	// Linux / BSD / macOS: xorriso emulando mkisofs.
	if _, err := exec.LookPath("xorriso"); err == nil {
		args := append([]string{
			"-as", "mkisofs", "-quiet", "-output", output,
			"-volid", "cidata", "-joliet", "-rock",
		}, files...)
		return runISOTool(ctx, executor, "xorriso", args...)
	}

	// macOS: hdiutil trabaja sobre un directorio.
	if runtime.GOOS == "darwin" {
		if _, err := exec.LookPath("hdiutil"); err == nil {
			tmpDir, err := os.MkdirTemp(workDir, "seed-*")
			if err != nil {
				return err
			}
			defer func() { _ = os.RemoveAll(tmpDir) }()
			for _, file := range files {
				data, err := os.ReadFile(file)
				if err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(tmpDir, filepath.Base(file)), data, 0o644); err != nil {
					return err
				}
			}
			return runISOTool(ctx, executor, "hdiutil",
				"makehybrid", "-iso", "-joliet",
				"-default-volume-name", "cidata",
				"-o", output, tmpDir,
			)
		}
	}

	// Windows: oscdimg (Windows ADK).
	if runtime.GOOS == "windows" {
		if _, err := exec.LookPath("oscdimg"); err == nil {
			tmpDir, err := os.MkdirTemp(workDir, "seed-*")
			if err != nil {
				return err
			}
			defer func() { _ = os.RemoveAll(tmpDir) }()
			for _, file := range files {
				data, err := os.ReadFile(file)
				if err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(tmpDir, filepath.Base(file)), data, 0o644); err != nil {
					return err
				}
			}
			return runISOTool(ctx, executor, "oscdimg",
				"-m", "-o", "-u2", "-lCIDATA", tmpDir, output,
			)
		}
	}

	return fmt.Errorf("no hay herramienta para crear la ISO semilla; instala genisoimage, mkisofs, xorriso u oscdimg")
}

func runISOTool(ctx context.Context, executor ports.Executor, tool string, args ...string) error {
	_, stderr, err := executor.Execute(ctx, tool, args...)
	if err != nil {
		detail := string(stderr)
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("%s falló: %s", tool, detail)
	}
	return nil
}
