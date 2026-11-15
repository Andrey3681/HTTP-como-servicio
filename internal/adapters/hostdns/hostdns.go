// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

// Package hostdns configura el resolver del anfitrión para la zona cloud.local.
package hostdns

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/ports"
)

// Adapter ejecuta el configurador del DNS del anfitrión a través de pkexec.
type Adapter struct {
	executor ports.Executor
	script   string
}

// New crea el configurador apuntando al script del repositorio.
func New(executor ports.Executor) *Adapter {
	script, err := filepath.Abs(filepath.Join("scripts", "configurar-dns-anfitrion.sh"))
	if err != nil {
		script = filepath.Join("scripts", "configurar-dns-anfitrion.sh")
	}
	return &Adapter{executor: executor, script: script}
}

// Configure aplica la configuración con pkexec (diálogo de autorización) o
// directamente si ya se ejecuta como administrador.
func (a *Adapter) Configure(ctx context.Context, iface, dnsIP string) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("la configuración automática del DNS del anfitrión solo está disponible en Linux con systemd-resolved")
	}
	command := a.script
	args := []string{iface, dnsIP}
	if _, err := exec.LookPath("pkexec"); err == nil {
		command = "pkexec"
		args = append([]string{a.script}, args...)
	}

	_, stderr, err := a.executor.Execute(ctx, command, args...)
	if err != nil {
		detail := strings.TrimSpace(string(stderr))
		if detail == "" {
			detail = "autorización cancelada o no disponible"
		}
		return fmt.Errorf("%s", detail)
	}
	return nil
}
