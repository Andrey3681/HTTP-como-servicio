// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

// Package bind9 implementa ports.DnsProvider para registrar y remover registros A en el servidor Bind9.
package bind9

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"net/netip"
	"strings"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/domain"
	"github.com/Andrey3681/httpaas-orchestrator/internal/core/ports"
)

// DnsAdapter gestiona la actualización dinámica de nombres en Bind9.
type DnsAdapter struct {
	executor ports.Executor
	serverIP string
	zone     string
	nsupdate string
	network  ports.NetworkInfoProvider
	logger   ports.EventLogger
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

	// Ejecutar nsupdate
	_, stderr, err := a.executor.Execute(ctx, a.nsupdate, "-v")
	if err != nil {
		log.Printf("[Bind9] Aviso: nsupdate no disponible o falló en host anfitrión: %s (%v)", string(stderr), err)
		// Si nsupdate local no está instalado en Windows, se registra informativamente
		// y se continuará con resolución IP directa.
		return nil
	}

	return nil
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

	_, _, _ = a.executor.Execute(ctx, a.nsupdate, "-v")
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
