// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package seediso_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/Andrey3681/httpaas-orchestrator/internal/adapters/localproc"
	"github.com/Andrey3681/httpaas-orchestrator/internal/adapters/seediso"
)

func TestBuild_NoFilesReturnsError(t *testing.T) {
	executor := localproc.NewProcessExecutor()
	err := seediso.Build(context.Background(), executor, t.TempDir(), filepath.Join(t.TempDir(), "seed.iso"), nil)
	if err == nil {
		t.Fatalf("expected error for empty file list, got nil")
	}
}

func TestBuild_GeneratesValidNoCloudISO(t *testing.T) {
	tempDir := t.TempDir()
	userDataContent := []byte("#cloud-config\nhostname: ns-server\n")
	metaDataContent := []byte("instance-id: iid-ns-01\nlocal-hostname: ns-server\n")

	userDataFile := filepath.Join(tempDir, "user-data")
	metaDataFile := filepath.Join(tempDir, "meta-data")
	if err := os.WriteFile(userDataFile, userDataContent, 0o644); err != nil {
		t.Fatalf("failed writing user-data: %v", err)
	}
	if err := os.WriteFile(metaDataFile, metaDataContent, 0o644); err != nil {
		t.Fatalf("failed writing meta-data: %v", err)
	}

	outISO := filepath.Join(tempDir, "test-seed.iso")
	executor := localproc.NewProcessExecutor()

	if err := seediso.Build(context.Background(), executor, tempDir, outISO, []string{userDataFile, metaDataFile}); err != nil {
		t.Fatalf("Build() failed: %v", err)
	}

	raw, err := os.ReadFile(outISO)
	if err != nil {
		t.Fatalf("failed reading generated ISO: %v", err)
	}
	if len(raw) < 20*2048 {
		t.Fatalf("ISO too small: %d bytes", len(raw))
	}

	// Verify Primary Volume Descriptor at sector 16.
	pvd := raw[16*2048 : 17*2048]
	if string(pvd[1:6]) != "CD001" {
		t.Fatalf("expected CD001 signature at sector 16, got %q", string(pvd[1:6]))
	}
	volID := bytes.TrimSpace(pvd[40:72])
	if !bytes.EqualFold(volID, []byte("cidata")) {
		t.Fatalf("expected volume ID cidata, got %q", string(volID))
	}

	// Verify exact lowercase filenames and file payloads are present in the image.
	if !bytes.Contains(raw, []byte("user-data")) {
		t.Fatalf("expected lowercase 'user-data' filename in ISO image")
	}
	if !bytes.Contains(raw, []byte("meta-data")) {
		t.Fatalf("expected lowercase 'meta-data' filename in ISO image")
	}
	if !bytes.Contains(raw, userDataContent) {
		t.Fatalf("expected user-data payload in ISO image")
	}
	if !bytes.Contains(raw, metaDataContent) {
		t.Fatalf("expected meta-data payload in ISO image")
	}
}

func TestBuild_ConcurrentExecution(t *testing.T) {
	tempDir := t.TempDir()
	userDataFile := filepath.Join(tempDir, "user-data")
	metaDataFile := filepath.Join(tempDir, "meta-data")
	if err := os.WriteFile(userDataFile, []byte("user-data-payload"), 0o644); err != nil {
		t.Fatalf("failed writing user-data: %v", err)
	}
	if err := os.WriteFile(metaDataFile, []byte("meta-data-payload"), 0o644); err != nil {
		t.Fatalf("failed writing meta-data: %v", err)
	}

	executor := localproc.NewProcessExecutor()
	const workers = 4
	var wg sync.WaitGroup
	errs := make(chan error, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			outISO := filepath.Join(tempDir, fmt.Sprintf("seed-%d.iso", idx))
			if err := seediso.Build(context.Background(), executor, tempDir, outISO, []string{userDataFile, metaDataFile}); err != nil {
				errs <- err
			}
		}(i)
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent Build() failed: %v", err)
	}
}
