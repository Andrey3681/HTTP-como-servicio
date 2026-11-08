// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package localproc_test

import (
	"context"
	"testing"
	"time"

	"github.com/Andrey3681/httpaas-orchestrator/internal/adapters/localproc"
)

func TestProcessExecutor_Execute(t *testing.T) {
	executor := localproc.NewProcessExecutor()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Probar un comando universal disponible en Go
	stdout, _, err := executor.Execute(ctx, "go", "version")
	if err != nil {
		t.Fatalf("unexpected error running 'go version': %v", err)
	}

	if len(stdout) == 0 {
		t.Errorf("expected non-empty stdout from 'go version'")
	}
}
