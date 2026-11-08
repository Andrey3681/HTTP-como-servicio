// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

// Package vbox implementa el adaptador de VirtualBox controlando VBoxManage con serialización de concurrencia.
package vbox

import (
	"fmt"
	"net/netip"
	"regexp"
	"strings"
)

var ipValueRegex = regexp.MustCompile(`Value:\s*([0-9]+\.[0-9]+\.[0-9]+\.[0-9]+)`)

// ParseGuestIP parsea la salida de 'VBoxManage guestproperty get <vm> /VirtualBox/GuestInfo/Net/0/V4/IP'.
func ParseGuestIP(output string) (netip.Addr, error) {
	if strings.Contains(output, "No value set!") {
		return netip.Addr{}, fmt.Errorf("la propiedad de IP aún no está asignada")
	}

	matches := ipValueRegex.FindStringSubmatch(output)
	if len(matches) < 2 {
		return netip.Addr{}, fmt.Errorf("no se encontró una dirección IP válida en la salida: %s", output)
	}

	addr, err := netip.ParseAddr(matches[1])
	if err != nil {
		return netip.Addr{}, fmt.Errorf("IP inválida parseada '%s': %w", matches[1], err)
	}

	return addr, nil
}
