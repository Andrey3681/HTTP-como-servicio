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

// ParseVMMac extrae la MAC del primer adaptador de red de
// 'showvminfo --machinereadable'.
func ParseVMMac(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		for _, key := range []string{"macaddress1", `"macaddress1"`} {
			if strings.HasPrefix(line, key+"=") {
				return strings.Trim(strings.TrimPrefix(line, key+"="), `"`)
			}
		}
	}
	return ""
}

// ParseDhcpLeaseIP extrae la IP de la salida de 'dhcpserver findlease'.
func ParseDhcpLeaseIP(output string) (netip.Addr, error) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "IP Address:") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, "IP Address:"))
		addr, err := netip.ParseAddr(value)
		if err != nil || !addr.Is4() || addr.IsLoopback() {
			return netip.Addr{}, fmt.Errorf("IP inválida en la concesión DHCP: %q", value)
		}
		return addr, nil
	}
	return netip.Addr{}, fmt.Errorf("sin concesión DHCP en la salida: %s", strings.TrimSpace(output))
}

// ParseNeighborIP busca en la tabla de vecinos la IP asociada a una MAC.
func ParseNeighborIP(output, mac string) (netip.Addr, error) {
	want := normalizeMAC(mac)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[1] != "lladdr" {
			continue
		}
		if normalizeMAC(fields[2]) != want {
			continue
		}
		addr, err := netip.ParseAddr(fields[0])
		if err == nil && addr.Is4() && !addr.IsLoopback() {
			return addr, nil
		}
	}
	return netip.Addr{}, fmt.Errorf("la MAC %s no aparece en la tabla de vecinos", mac)
}

// ParseWindowsARP busca en "arp -a" de Windows la IP asociada a una MAC.
func ParseWindowsARP(output, mac string) (netip.Addr, error) {
	want := normalizeMAC(mac)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if normalizeMAC(fields[1]) != want {
			continue
		}
		addr, err := netip.ParseAddr(fields[0])
		if err == nil && addr.Is4() && !addr.IsLoopback() {
			return addr, nil
		}
	}
	return netip.Addr{}, fmt.Errorf("la MAC %s no aparece en la tabla ARP", mac)
}

// normalizeMAC unifica el formato de las MAC para compararlas.
func normalizeMAC(mac string) string {
	clean := strings.ToLower(strings.NewReplacer(":", "", "-", "").Replace(mac))
	if len(clean) != 12 {
		return strings.ToLower(mac)
	}
	return clean[0:2] + ":" + clean[2:4] + ":" + clean[4:6] + ":" +
		clean[6:8] + ":" + clean[8:10] + ":" + clean[10:12]
}

// ParseHostOnlyInterfaceByIP localiza el nombre de la interfaz host-only con la IP indicada.
func ParseHostOnlyInterfaceByIP(output, ip string) string {
	var name string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Name:"):
			name = strings.TrimSpace(strings.TrimPrefix(line, "Name:"))
		case strings.HasPrefix(line, "IPAddress:"):
			if strings.TrimSpace(strings.TrimPrefix(line, "IPAddress:")) == ip && name != "" {
				return name
			}
		}
	}
	return ""
}

// ParseFirstProjectInterface retorna el nombre de la primera interfaz host-only
// que no sea la red por defecto de VirtualBox (192.168.56.1).
func ParseFirstProjectInterface(output string) string {
	var name string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Name:"):
			name = strings.TrimSpace(strings.TrimPrefix(line, "Name:"))
		case strings.HasPrefix(line, "IPAddress:"):
			ip := strings.TrimSpace(strings.TrimPrefix(line, "IPAddress:"))
			if name != "" && ip != "" && ip != "192.168.56.1" {
				return name
			}
		}
	}
	return ""
}

// ParseMachineValue extrae el valor de una clave en la salida --machinereadable.
func ParseMachineValue(output, key string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, key+"=") {
			continue
		}
		return strings.Trim(strings.TrimPrefix(line, key+"="), `"`)
	}
	return ""
}
