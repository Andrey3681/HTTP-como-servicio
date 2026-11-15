// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package domain

import (
	"encoding/binary"
	"fmt"
	"math/bits"
	"net/netip"
	"strings"
)

// NetworkConfig describe la red host-only real del laboratorio. Se deriva de la
// dirección que VirtualBox asigna a la interfaz, sin imponer una subred fija.
type NetworkConfig struct {
	Interface string `json:"interfaz"`
	HostIP    string `json:"ip_anfitrion"`
	Netmask   string `json:"mascara"`
	CIDR      string `json:"cidr"`
	DNSIP     string `json:"ip_dns"`
	BaseIP    string `json:"ip_plantilla"`
	DHCPLower string `json:"dhcp_inicio"`
	DHCPUpper string `json:"dhcp_fin"`
}

// DeriveNetwork calcula la configuración del laboratorio a partir de la IP y la
// máscara de la interfaz host-only (se admite /24, el valor habitual de VirtualBox).
func DeriveNetwork(iface, hostIP, netmask string) (NetworkConfig, error) {
	addr, err := netip.ParseAddr(strings.TrimSpace(hostIP))
	if err != nil || !addr.Is4() {
		return NetworkConfig{}, fmt.Errorf("dirección de anfitrión inválida: %q", hostIP)
	}
	bits, err := netmaskBits(netmask)
	if err != nil {
		return NetworkConfig{}, err
	}
	if bits != 24 {
		return NetworkConfig{}, fmt.Errorf("la red host-only debe ser /24; se recibió /%d", bits)
	}

	masked := netip.PrefixFrom(addr, bits).Masked()
	base := masked.Addr().As4()
	return NetworkConfig{
		Interface: iface,
		HostIP:    addr.String(),
		Netmask:   netmask,
		CIDR:      masked.String(),
		DNSIP:     netip.AddrFrom4([4]byte{base[0], base[1], base[2], 10}).String(),
		BaseIP:    netip.AddrFrom4([4]byte{base[0], base[1], base[2], 11}).String(),
		DHCPLower: netip.AddrFrom4([4]byte{base[0], base[1], base[2], 100}).String(),
		DHCPUpper: netip.AddrFrom4([4]byte{base[0], base[1], base[2], 199}).String(),
	}, nil
}

// netmaskBits convierte una máscara decimal en longitud de prefijo.
func netmaskBits(mask string) (int, error) {
	addr, err := netip.ParseAddr(strings.TrimSpace(mask))
	if err != nil || !addr.Is4() {
		return 0, fmt.Errorf("máscara inválida: %q", mask)
	}
	raw := addr.As4()
	value := binary.BigEndian.Uint32(raw[:])
	length := bits.LeadingZeros32(^value)
	if length > 32 || value != ^uint32(0)<<(32-length) {
		return 0, fmt.Errorf("máscara no contigua: %q", mask)
	}
	return length, nil
}
