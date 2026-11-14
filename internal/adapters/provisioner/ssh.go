// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package provisioner

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/domain"
)

// EnsureSSHKey crea el par de claves del laboratorio si no existe y retorna la
// clave pública para inyectarla en las semillas cloud-init.
func (p *Provisioner) EnsureSSHKey(ctx context.Context, cfg domain.SetupConfig) (string, error) {
	keyPath := cfg.SSHKeyPath
	if keyPath == "" {
		keyPath = filepath.Join(cfg.SSHDir, "id_ed25519")
	}
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
		return "", err
	}
	if _, err := os.Stat(keyPath); err != nil {
		if _, stderr, err := p.executor.Execute(ctx, "ssh-keygen",
			"-t", "ed25519", "-N", "", "-C", "httpaas", "-f", keyPath,
		); err != nil {
			return "", fmt.Errorf("no se pudo generar la clave SSH: %w (%s)", err, strings.TrimSpace(string(stderr)))
		}
	}
	raw, err := os.ReadFile(keyPath + ".pub")
	if err != nil {
		return "", fmt.Errorf("no se pudo leer la clave pública: %w", err)
	}
	return strings.TrimSpace(string(raw)), nil
}

// WaitSSH espera a que el invitado acepte conexiones SSH con la clave indicada.
func (p *Provisioner) WaitSSH(ctx context.Context, host, user, keyPath string, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if _, err := p.RunSSH(ctx, host, user, keyPath, "true"); err == nil {
			return nil
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
	return fmt.Errorf("SSH no disponible en %s: %w", host, lastErr)
}

// RunSSH ejecuta un comando en el invitado y retorna salida y errores juntos.
func (p *Provisioner) RunSSH(ctx context.Context, host, user, keyPath, command string) (string, error) {
	args := []string{
		"-i", keyPath,
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=5",
		"-o", "ServerAliveInterval=15",
		"-o", "ServerAliveCountMax=3",
		user + "@" + host,
		command,
	}
	stdout, stderr, err := p.executor.Execute(ctx, "ssh", args...)
	out := string(stdout) + string(stderr)
	if err != nil {
		return out, fmt.Errorf("ssh %s: %w", host, err)
	}
	return out, nil
}

// RunScriptSSH copia un script al invitado por SSH y lo ejecuta como root con
// los argumentos indicados, retornando la salida combinada del comando.
func (p *Provisioner) RunScriptSSH(ctx context.Context, cfg domain.SetupConfig, ip, localScript string, args ...string) (string, error) {
	data, err := os.ReadFile(localScript)
	if err != nil {
		return "", fmt.Errorf("script de invitado no encontrado: %s: %w", localScript, err)
	}
	keyPath := cfg.SSHKeyPath
	if keyPath == "" {
		keyPath = filepath.Join(cfg.SSHDir, "id_ed25519")
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	remote := "tmp=/tmp/httpaas-" + filepath.Base(localScript) +
		"; echo '" + encoded + "' | base64 -d > \"$tmp\" && sudo bash \"$tmp\""
	for _, arg := range args {
		remote += " " + shellQuote(arg)
	}
	remote += "; rc=$?; rm -f \"$tmp\"; exit $rc"

	var lastOut string
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		out, err := p.RunSSH(ctx, ip, cfg.GuestUser, keyPath, remote)
		if err == nil {
			return out, nil
		}
		lastOut, lastErr = out, err
		// Solo se reintenta ante fallos de conexión, no ante errores del script.
		if !strings.Contains(out, "Connection timed out") &&
			!strings.Contains(out, "Connection refused") &&
			!strings.Contains(out, "No route to host") &&
			!strings.Contains(out, "exit status 255") {
			return out, err
		}
		select {
		case <-ctx.Done():
			return out, ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
	return lastOut, lastErr
}

// shellQuote envuelve un argumento para que el shell remoto lo trate literal.
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
