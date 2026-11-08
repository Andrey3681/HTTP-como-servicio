// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package domain

import (
	"fmt"
	"net/netip"
	"strings"
)

// DnsRecord representa una entrada en el servidor DNS Bind9.
type DnsRecord struct {
	Name string `json:"nombre"`
	Zone string `json:"zona"`
	Type string `json:"tipo"`
	IP   string `json:"ip"`
	TTL  int    `json:"ttl"`
}

// NewARecord construye y valida un registro de tipo A para la zona DNS local.
func NewARecord(name string, zone string, ip netip.Addr, ttl int) (*DnsRecord, error) {
	if err := ValidateHostname(name); err != nil {
		return nil, err
	}

	if !ip.IsValid() || !ip.Is4() {
		return nil, fmt.Errorf("la dirección IP '%s' no es una IPv4 válida", ip.String())
	}

	cleanZone := strings.Trim(zone, ".") + "."
	cleanName := strings.Trim(name, ".")
	fqdn := fmt.Sprintf("%s.%s", cleanName, cleanZone)

	if ttl <= 0 {
		ttl = 300 // 5 minutos por defecto
	}

	return &DnsRecord{
		Name: fqdn,
		Zone: cleanZone,
		Type: "A",
		IP:   ip.String(),
		TTL:  ttl,
	}, nil
}
