// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

// Package bind9 implementa ports.DnsProvider para registrar y remover registros A en el servidor Bind9.
package bind9

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"net/netip"
	"os"
	"strings"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/domain"
	"github.com/Andrey3681/httpaas-orchestrator/internal/core/ports"
)

// DnsAdapter gestiona la actualización dinámica de nombres en Bind9.
type DnsAdapter struct {
	executor  ports.Executor
	serverIP  string
	zone      string
	nsupdate  string
	network   ports.NetworkInfoProvider
	logger    ports.EventLogger
	guestUser func() string
	keyPath   string
}

// ConfigureSSH configura el usuario y la clave privada SSH del laboratorio
// para ejecutar nsupdate y dig dentro de la máquina DNS cuando el anfitrión
// no dispone de las utilidades de Bind9 instaladas localmente.
func (a *DnsAdapter) ConfigureSSH(guestUser func() string, keyPath string) {
	a.guestUser = guestUser
	a.keyPath = keyPath
}

// NewDnsAdapter crea una nueva instancia de DnsAdapter apuntando al servidor Bind9.
func NewDnsAdapter(executor ports.Executor, serverIP string, zone string, logger ports.EventLogger) *DnsAdapter {
	return &DnsAdapter{
		executor: executor,
		serverIP: serverIP,
		zone:     strings.Trim(zone, ".") + ".",
		nsupdate: "nsupdate",
		logger:   logger,
	}
}

// NewDnsAdapterWithNetwork deriva la IP del servidor DNS de la red host-only real.
func NewDnsAdapterWithNetwork(executor ports.Executor, zone string, logger ports.EventLogger, network ports.NetworkInfoProvider) *DnsAdapter {
	adapter := NewDnsAdapter(executor, "", zone, logger)
	adapter.network = network
	return adapter
}

// address resuelve la IP del servidor DNS: fija si se indicó o derivada de la red.
func (a *DnsAdapter) address() (string, error) {
	if a.serverIP != "" {
		return a.serverIP, nil
	}
	if a.network != nil {
		if cfg := a.network.Network(); cfg.DNSIP != "" {
			return cfg.DNSIP, nil
		}
	}
	return "", fmt.Errorf("el servidor DNS aún no está disponible: completa la preparación del entorno")
}

func (a *DnsAdapter) log(level, message string, fields map[string]any) {
	if a.logger != nil {
		a.logger.Append(level, "bind9", message, fields)
	}
}

// AddRecord registra dinámicamente un registro tipo A mediante el protocolo RFC 2136 con nsupdate.
func (a *DnsAdapter) AddRecord(ctx context.Context, record domain.DnsRecord) error {
	log.Printf("[Bind9] Registrando host '%s' -> %s en zona %s", record.Name, record.IP, a.zone)
	a.log("INFO", "Registrando registro DNS", map[string]any{"nombre": record.Name, "ip": record.IP, "zona": a.zone})

	var script bytes.Buffer
	server, err := a.address()
	if err != nil {
		return err
	}
	fmt.Fprintf(&script, "server %s\n", server)
	fmt.Fprintf(&script, "zone %s\n", a.zone)
	fmt.Fprintf(&script, "update delete %s A\n", record.Name)
	fmt.Fprintf(&script, "update add %s %d A %s\n", record.Name, record.TTL, record.IP)
	fmt.Fprintf(&script, "send\n")

	if err := a.applyUpdate(ctx, script.String()); err != nil {
		return err
	}
	a.log("INFO", "Registro DNS aplicado", map[string]any{"nombre": record.Name, "ip": record.IP})
	return nil
}

// applyUpdate envía el guion RFC 2136 a nsupdate mediante un archivo temporal.
func (a *DnsAdapter) applyUpdate(ctx context.Context, script string) error {
	file, err := os.CreateTemp("", "httpaas-nsupdate-*.txt")
	if err != nil {
		return fmt.Errorf("no se pudo preparar la actualización DNS: %w", err)
	}
	path := file.Name()
	defer func() { _ = os.Remove(path) }()
	if _, err := file.WriteString(script); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}

	stdout, stderr, err := a.executor.Execute(ctx, a.nsupdate, "-v", path)
	if err != nil {
		if a.keyPath != "" {
			if server, addrErr := a.address(); addrErr == nil {
				encoded := base64.StdEncoding.EncodeToString([]byte(script))
				remote := "echo '" + encoded + "' | base64 -d | nsupdate -v"
				sshOut, sshErrOut, sshErr := a.executor.Execute(ctx, "ssh", a.sshArgs(server, remote)...)
				if sshErr == nil {
					return nil
				}
				stdout, stderr, err = sshOut, sshErrOut, sshErr
			}
		}
		detail := strings.TrimSpace(string(stderr))
		if detail == "" {
			detail = strings.TrimSpace(string(stdout))
		}
		return fmt.Errorf("nsupdate falló: %s (%w)", detail, err)
	}
	return nil
}

func (a *DnsAdapter) sshArgs(server, command string) []string {
	user := "debian"
	if a.guestUser != nil {
		if value := a.guestUser(); value != "" {
			user = value
		}
	}
	return []string{
		"-i", a.keyPath,
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=10",
		user + "@" + server,
		command,
	}
}

// RemoveRecord elimina el registro A del host en la zona DNS.
func (a *DnsAdapter) RemoveRecord(ctx context.Context, hostname string) error {
	cleanName := strings.Trim(hostname, ".")
	fqdn := fmt.Sprintf("%s.%s", cleanName, a.zone)

	log.Printf("[Bind9] Removiendo registro '%s' de zona %s", fqdn, a.zone)
	a.log("INFO", "Eliminando registro DNS", map[string]any{"nombre": fqdn, "zona": a.zone})

	var script bytes.Buffer
	server, err := a.address()
	if err != nil {
		return err
	}
	fmt.Fprintf(&script, "server %s\n", server)
	fmt.Fprintf(&script, "zone %s\n", a.zone)
	fmt.Fprintf(&script, "update delete %s A\n", fqdn)
	fmt.Fprintf(&script, "send\n")

	if err := a.applyUpdate(ctx, script.String()); err != nil {
		return err
	}
	return nil
}

// Resolve verifica la resolución DNS del nombre de host.
func (a *DnsAdapter) Resolve(ctx context.Context, hostname string) (netip.Addr, error) {
	cleanHost := strings.TrimSuffix(hostname, ".")
	if !strings.HasSuffix(cleanHost, a.zone) {
		cleanHost = fmt.Sprintf("%s.%s", cleanHost, strings.TrimSuffix(a.zone, "."))
	}

	server, err := a.address()
	if err != nil {
		return netip.Addr{}, err
	}
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{}
			return d.DialContext(ctx, "udp", net.JoinHostPort(server, "53"))
		},
	}

	ips, err := resolver.LookupHost(ctx, cleanHost)
	if err != nil || len(ips) == 0 {
		return netip.Addr{}, fmt.Errorf("no se pudo resolver el nombre '%s' en servidor %s: %w", cleanHost, server, err)
	}

	return netip.ParseAddr(ips[0])
}

// ListRecords consulta los registros A de la zona administrada mediante transferencia AXFR (dig).
// Si la consulta no está disponible, retorna una lista vacía sin error para preservar el contrato de la API.
func (a *DnsAdapter) ListRecords(ctx context.Context) ([]domain.DnsRecord, error) {
	server, err := a.address()
	if err != nil {
		return []domain.DnsRecord{}, nil
	}
	stdout, _, err := a.executor.Execute(ctx, "dig", "@"+server, a.zone, "AXFR", "+noall", "+answer")
	if (err != nil || len(bytes.TrimSpace(stdout)) == 0) && a.keyPath != "" {
		remote := fmt.Sprintf("dig @%s %s AXFR +noall +answer", server, a.zone)
		stdout, _, err = a.executor.Execute(ctx, "ssh", a.sshArgs(server, remote)...)
	}
	if err != nil || len(bytes.TrimSpace(stdout)) == 0 {
		return []domain.DnsRecord{}, nil
	}

	var records []domain.DnsRecord
	for _, line := range strings.Split(string(stdout), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		// nombre TTL [clase] tipo datos...
		rrType := strings.ToUpper(fields[3])
		if rrType != "A" {
			// Some dig outputs: name TTL class type data
			if len(fields) >= 5 && strings.ToUpper(fields[2]) == "IN" {
				rrType = strings.ToUpper(fields[3])
			}
		}
		if rrType != "A" {
			continue
		}
		ip := fields[len(fields)-1]
		ttl := 300
		_, _ = fmt.Sscanf(fields[1], "%d", &ttl)
		records = append(records, domain.DnsRecord{
			Name: fields[0],
			Zone: a.zone,
			Type: "A",
			IP:   ip,
			TTL:  ttl,
		})
	}
	return records, nil
}
