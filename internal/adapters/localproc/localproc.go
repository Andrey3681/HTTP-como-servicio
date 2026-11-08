// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

// Package localproc implementa el puerto ports.Executor ejecutando procesos en el host local mediante os/exec.
package localproc

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/ports"
)

// ProcessExecutor ejecuta comandos de sistema local mediante os/exec.
type ProcessExecutor struct{}

// NewProcessExecutor crea una nueva instancia de ProcessExecutor.
func NewProcessExecutor() ports.Executor {
	return &ProcessExecutor{}
}

// Execute ejecuta el comando proporcionado con argumentos respetando el contexto de cancelación.
func (e *ProcessExecutor) Execute(ctx context.Context, command string, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, command, args...)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	if err != nil {
		return stdoutBuf.Bytes(), stderrBuf.Bytes(), fmt.Errorf("fallo al ejecutar '%s %v': %w (stderr: %s)",
			command, args, err, stderrBuf.String())
	}

	return stdoutBuf.Bytes(), stderrBuf.Bytes(), nil
}
