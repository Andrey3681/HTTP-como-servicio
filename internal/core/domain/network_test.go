// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package domain_test

import (
	"testing"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/domain"
)

func TestDeriveNetwork(t *testing.T) {
	network, err := domain.DeriveNetwork("vboxnet1", "192.168.57.1", "255.255.255.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if network.Interface != "vboxnet1" {
		t.Errorf("unexpected interface: %q", network.Interface)
	}
	if network.CIDR != "192.168.57.0/24" {
		t.Errorf("unexpected CIDR: %q", network.CIDR)
	}
	if network.DNSIP != "192.168.57.10" {
		t.Errorf("unexpected DNS IP: %q", network.DNSIP)
	}
	if network.DHCPLower != "192.168.57.100" || network.DHCPUpper != "192.168.57.199" {
		t.Errorf("unexpected DHCP range: %q-%q", network.DHCPLower, network.DHCPUpper)
	}
}

func TestDeriveNetworkUsesTheInterfaceSubnet(t *testing.T) {
	network, err := domain.DeriveNetwork("vboxnet2", "10.20.30.1", "255.255.255.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if network.DNSIP != "10.20.30.10" || network.CIDR != "10.20.30.0/24" {
		t.Errorf("unexpected derived network: %+v", network)
	}
}

func TestDeriveNetworkRejectsInvalidInput(t *testing.T) {
	if _, err := domain.DeriveNetwork("vboxnet1", "no-es-ip", "255.255.255.0"); err == nil {
		t.Error("expected an error for an invalid host IP")
	}
	if _, err := domain.DeriveNetwork("vboxnet1", "192.168.57.1", "255.255.0.0"); err == nil {
		t.Error("expected an error for a non-/24 network")
	}
}
