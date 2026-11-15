// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

// Package hostdns configura el resolver del anfitrión para la zona cloud.local.
package hostdns

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/netip"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf16"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/ports"
)

// Adapter ejecuta el configurador del DNS del anfitrión a través de pkexec en
// Linux o mediante elevación UAC de PowerShell en Windows.
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

// Configure aplica la configuración con pkexec (diálogo de autorización en
// Linux) o con Start-Process -Verb RunAs (diálogo UAC en Windows).
func (a *Adapter) Configure(ctx context.Context, iface, dnsIP string) error {
	switch runtime.GOOS {
	case "windows":
		return a.configureWindows(ctx, iface, dnsIP)
	case "linux":
		return a.configureLinux(ctx, iface, dnsIP)
	default:
		return fmt.Errorf("la configuración automática del DNS del anfitrión solo está disponible en Linux y Windows")
	}
}

func (a *Adapter) configureLinux(ctx context.Context, iface, dnsIP string) error {
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

func (a *Adapter) configureWindows(ctx context.Context, iface, dnsIP string) error {
	addr, err := netip.ParseAddr(strings.TrimSpace(dnsIP))
	if err != nil || !addr.Is4() {
		return fmt.Errorf("dirección IP de DNS inválida: %q", dnsIP)
	}
	safeIface := strings.ReplaceAll(iface, "'", "''")
	safeDNS := addr.String()

	inner := fmt.Sprintf(
		"$ErrorActionPreference = 'Stop'; "+
			"Get-DnsClientNrptRule | Where-Object { $_.Namespace -contains '.cloud.local' } | Remove-DnsClientNrptRule -Force -ErrorAction SilentlyContinue; "+
			"Add-DnsClientNrptRule -Namespace '.cloud.local' -NameServers '%s'; "+
			"$adapter = Get-NetAdapter | Where-Object { $_.InterfaceDescription -eq '%s' -or $_.Name -eq '%s' }; "+
			"if ($adapter) { Set-DnsClientServerAddress -InterfaceIndex $adapter.InterfaceIndex -ServerAddresses ('%s') -ErrorAction SilentlyContinue }; "+
			"Clear-DnsClientCache",
		safeDNS, safeIface, safeIface, safeDNS,
	)
	encoded := encodePowerShellUTF16LE(inner)
	outer := fmt.Sprintf(
		"$p = Start-Process powershell -Verb RunAs -WindowStyle Hidden -Wait -PassThru -ArgumentList @('-NoProfile','-NonInteractive','-EncodedCommand','%s'); "+
			"if (-not $p -or $p.ExitCode -ne 0) { exit 1 }",
		encoded,
	)

	_, stderr, err := a.executor.Execute(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", outer)
	if err != nil {
		detail := strings.TrimSpace(string(stderr))
		if detail == "" {
			detail = "autorización UAC cancelada o no disponible"
		}
		return fmt.Errorf("%s", detail)
	}
	return nil
}

func encodePowerShellUTF16LE(script string) string {
	u16 := utf16.Encode([]rune(script))
	buf := make([]byte, len(u16)*2)
	for i, v := range u16 {
		buf[i*2] = byte(v)
		buf[i*2+1] = byte(v >> 8)
	}
	return base64.StdEncoding.EncodeToString(buf)
}
