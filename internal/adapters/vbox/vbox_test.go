// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package vbox_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Andrey3681/httpaas-orchestrator/internal/adapters/vbox"
)

func TestParseGuestIP(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		expectIP  string
		expectErr bool
	}{
		{
			name:      "Valid IP output",
			output:    "Value: 192.168.10.30\n",
			expectIP:  "192.168.10.30",
			expectErr: false,
		},
		{
			name:      "No value set",
			output:    "No value set!\n",
			expectErr: true,
		},
		{
			name:      "Empty output",
			output:    "",
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ip, err := vbox.ParseGuestIP(tc.output)
			if tc.expectErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.expectErr {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if ip.String() != tc.expectIP {
					t.Errorf("expected IP '%s', got '%s'", tc.expectIP, ip.String())
				}
			}
		})
	}
}

const hostOnlyOutput = `Name:            vboxnet1
GUID:            effffff9-dae8-4abf-8000-0a0027000000
DHCP:            Disabled
IPAddress:       192.168.10.1
NetworkMask:     255.255.255.0
`

type mockExecutor struct {
	commandsRun [][]string
	responses   map[string]string
}

func (m *mockExecutor) Execute(ctx context.Context, command string, args ...string) ([]byte, []byte, error) {
	full := append([]string{command}, args...)
	m.commandsRun = append(m.commandsRun, full)
	if m.responses != nil {
		if response, ok := m.responses[strings.Join(full, " ")]; ok {
			return []byte(response), nil, nil
		}
	}
	return []byte("OK"), nil, nil
}

func TestHypervisorAdapter_CreateInstanceFromBase(t *testing.T) {
	mockExec := &mockExecutor{responses: map[string]string{
		"VBoxManage list hostonlyifs":                      hostOnlyOutput,
		"VBoxManage showvminfo paisajes --machinereadable": "CfgFile=\"/home/user/VirtualBox VMs/paisajes/paisajes.vbox\"\n",
	}}
	adapter := vbox.NewHypervisorAdapter(mockExec, "cloud.local", "192.168.10.1", nil)

	inst, err := adapter.CreateInstanceFromBase(context.Background(), "paisajes", "/media/ApacheServer.vdi")
	if err != nil {
		t.Fatalf("unexpected error creating instance: %v", err)
	}

	if inst.Name != "paisajes" {
		t.Errorf("expected name 'paisajes', got '%s'", inst.Name)
	}

	// createvm, list hostonlyifs, modifyvm, storagectl, showvminfo,
	// createmedium y storageattach
	if len(mockExec.commandsRun) != 7 {
		t.Fatalf("expected 7 commands executed, got %d", len(mockExec.commandsRun))
	}

	joined := make([]string, 0, len(mockExec.commandsRun))
	for _, cmd := range mockExec.commandsRun {
		joined = append(joined, strings.Join(cmd, " "))
	}
	all := strings.Join(joined, "\n")

	if !strings.Contains(all, "--nic1 hostonly") || !strings.Contains(all, "--hostonlyadapter1 vboxnet1") {
		t.Errorf("expected host-only networking, got:\n%s", all)
	}
	if !strings.Contains(all, "--diffparent /media/ApacheServer.vdi") {
		t.Errorf("expected differencing disk creation, got:\n%s", all)
	}
	if !strings.Contains(all, "--medium /home/user/VirtualBox VMs/paisajes/paisajes.vdi") {
		t.Errorf("expected the differencing disk to be attached, got:\n%s", all)
	}
}

func TestHypervisorAdapter_MissingHostOnlyNetwork(t *testing.T) {
	adapter := vbox.NewHypervisorAdapter(&mockExecutor{}, "cloud.local", "192.168.10.1", nil)

	_, err := adapter.CreateInstanceFromBase(context.Background(), "paisajes", "/media/ApacheServer.vdi")
	if err == nil {
		t.Fatal("expected an error when the host-only network is missing")
	}
	if !strings.Contains(err.Error(), "preparación inicial") {
		t.Errorf("expected an actionable error, got: %v", err)
	}
}

func TestParseMachineValue(t *testing.T) {
	output := "name=\"paisajes\"\nCfgFile=\"/home/u/VirtualBox VMs/paisajes/paisajes.vbox\"\n"
	if got := vbox.ParseMachineValue(output, "CfgFile"); got != "/home/u/VirtualBox VMs/paisajes/paisajes.vbox" {
		t.Errorf("unexpected CfgFile: %q", got)
	}
	if got := vbox.ParseMachineValue(output, "Missing"); got != "" {
		t.Errorf("expected empty value, got %q", got)
	}
}

func TestParseHostOnlyInterfaceByIP(t *testing.T) {
	if got := vbox.ParseHostOnlyInterfaceByIP(hostOnlyOutput, "192.168.10.1"); got != "vboxnet1" {
		t.Errorf("expected vboxnet1, got %q", got)
	}
	if got := vbox.ParseHostOnlyInterfaceByIP(hostOnlyOutput, "10.0.0.1"); got != "" {
		t.Errorf("expected empty result, got %q", got)
	}
}

func TestParseFirstProjectInterface(t *testing.T) {
	output := "Name: vboxnet0\nIPAddress: 192.168.56.1\n\nName: vboxnet1\nIPAddress: 192.168.57.1\n"
	if got := vbox.ParseFirstProjectInterface(output); got != "vboxnet1" {
		t.Errorf("expected vboxnet1, got %q", got)
	}
	if got := vbox.ParseFirstProjectInterface("Name: vboxnet0\nIPAddress: 192.168.56.1\n"); got != "" {
		t.Errorf("expected empty result for only the default network, got %q", got)
	}
}
