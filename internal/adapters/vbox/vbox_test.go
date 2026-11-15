// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package vbox_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	mu          sync.Mutex
	commandsRun [][]string
	responses   map[string]string
}

func (m *mockExecutor) Execute(ctx context.Context, command string, args ...string) ([]byte, []byte, error) {
	full := append([]string{command}, args...)
	m.mu.Lock()
	m.commandsRun = append(m.commandsRun, full)
	var response string
	var ok bool
	if m.responses != nil {
		response, ok = m.responses[strings.Join(full, " ")]
	}
	m.mu.Unlock()
	if ok {
		return []byte(response), nil, nil
	}
	return []byte("OK"), nil, nil
}

func TestParseDhcpLeaseIP(t *testing.T) {
	lease := "IP Address:  192.168.57.103\nMAC Address: 08:00:27:CC:8B:A4\nState:       acked\n"
	ip, err := vbox.ParseDhcpLeaseIP(lease)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ip.String() != "192.168.57.103" {
		t.Errorf("unexpected IP: %s", ip)
	}
	if _, err := vbox.ParseDhcpLeaseIP("VBoxManage: error: Could not find a lease"); err == nil {
		t.Error("expected an error without a lease")
	}
}

func TestParseVMMac(t *testing.T) {
	output := "name=\"paisajes\"\nmacaddress1=\"080027AABBCC\"\nmacaddress2=\"080027DDEEFF\"\n"
	if got := vbox.ParseVMMac(output); got != "080027AABBCC" {
		t.Errorf("unexpected MAC: %q", got)
	}
	if got := vbox.ParseVMMac("name=\"x\"\n"); got != "" {
		t.Errorf("expected empty MAC, got %q", got)
	}
}

func TestParseNeighborIP(t *testing.T) {
	output := "192.168.57.10 lladdr 08:00:27:19:68:f6 STALE\n192.168.57.105 lladdr 08:00:27:ad:09:bd REACHABLE\n"
	ip, err := vbox.ParseNeighborIP(output, "080027AD09BD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ip.String() != "192.168.57.105" {
		t.Errorf("unexpected IP: %s", ip)
	}
	if _, err := vbox.ParseNeighborIP(output, "080027000000"); err == nil {
		t.Error("expected an error for an unknown MAC")
	}
}

func TestHypervisorAdapter_GetIPAddressUsesArpTable(t *testing.T) {
	mockExec := &mockExecutor{responses: map[string]string{
		"VBoxManage list hostonlyifs":                      hostOnlyOutput,
		"VBoxManage showvminfo paisajes --machinereadable": "macaddress1=\"080027AABBCC\"\n",
		"ip neigh show dev vboxnet1":                       "192.168.57.107 lladdr 08:00:27:aa:bb:cc REACHABLE\n",
		"arp -a":                                           "Interfaz: 192.168.57.1 --- 0x9\n  192.168.57.107 08-00-27-aa-bb-cc dinámico\n",
	}}
	adapter := vbox.NewHypervisorAdapter(mockExec, "cloud.local", "", nil)

	ip, err := adapter.GetIPAddress(context.Background(), "paisajes")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ip.String() != "192.168.57.107" {
		t.Errorf("unexpected IP: %s", ip)
	}
}

func TestHypervisorAdapter_GetIPAddressUsesDhcpLease(t *testing.T) {
	mockExec := &mockExecutor{responses: map[string]string{
		"VBoxManage list hostonlyifs":                                                     hostOnlyOutput,
		"VBoxManage showvminfo paisajes --machinereadable":                                "macaddress1=\"080027AABBCC\"\n",
		"VBoxManage dhcpserver findlease --interface=vboxnet1 --mac-address=080027AABBCC": "IP Address:  192.168.10.103\nMAC Address: 08:00:27:AA:BB:CC\nState:       acked\n",
	}}
	adapter := vbox.NewHypervisorAdapter(mockExec, "cloud.local", "192.168.10.1", nil)

	ip, err := adapter.GetIPAddress(context.Background(), "paisajes")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ip.String() != "192.168.10.103" {
		t.Errorf("unexpected IP: %s", ip)
	}
}

func TestHypervisorAdapter_CreateInstanceAttachesSeed(t *testing.T) {
	dir := t.TempDir()
	keyPath := dir + "/id_ed25519"
	if err := os.WriteFile(keyPath+".pub", []byte("ssh-ed25519 AAAAC3Nza test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	mockExec := &mockExecutor{responses: map[string]string{
		"VBoxManage list hostonlyifs":                      hostOnlyOutput,
		"VBoxManage showvminfo paisajes --machinereadable": "CfgFile=\"/home/user/VMs/paisajes/paisajes.vbox\"\n",
	}}
	adapter := vbox.NewHypervisorAdapter(mockExec, "cloud.local", "192.168.10.1", nil)
	adapter.ConfigureInstanceSeed(func() string { return "jorge" }, keyPath, dir)

	if _, err := adapter.CreateInstanceFromBase(context.Background(), "paisajes", "/media/ApacheServer.vdi"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	userData, err := os.ReadFile(dir + "/instances/paisajes/user-data")
	if err != nil {
		t.Fatalf("no se generó la semilla: %v", err)
	}
	for _, want := range []string{
		"hostname: paisajes",
		"dhcp4: true",
		"dhcp-identifier: mac",
		"systemd-machine-id-setup",
		"ip addr flush dev enp0s3",
		"name: jorge",
		"ssh-ed25519",
	} {
		if !strings.Contains(string(userData), want) {
			t.Errorf("la semilla no contiene %q", want)
		}
	}
	if strings.Contains(string(userData), "enp0s8:") {
		t.Error("la semilla de instancia no debe configurar enp0s8")
	}
	attached := false
	for _, call := range mockExec.commandsRun {
		line := strings.Join(call, " ")
		if strings.Contains(line, "storageattach paisajes") && strings.Contains(line, "--port 1") && strings.Contains(line, "dvddrive") {
			attached = true
		}
	}
	if !attached {
		t.Error("no se conectó la ISO semilla de la instancia")
	}
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
	if !strings.Contains(filepath.ToSlash(all), "--medium /home/user/VirtualBox VMs/paisajes/paisajes.vdi") {
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

func TestParseWindowsARP(t *testing.T) {
	output := "Interfaz: 192.168.57.1 --- 0x9\r\n" +
		"  Dirección de Internet          Dirección física      Tipo\r\n" +
		"  192.168.57.10          08-00-27-19-68-f6     dinámico\r\n" +
		"  192.168.57.105         08-00-27-ad-09-bd     dinámico\r\n"
	ip, err := vbox.ParseWindowsARP(output, "080027AD09BD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ip.String() != "192.168.57.105" {
		t.Errorf("unexpected IP: %s", ip)
	}
	if _, err := vbox.ParseWindowsARP(output, "080027000000"); err == nil {
		t.Error("expected an error for an unknown MAC")
	}
}
