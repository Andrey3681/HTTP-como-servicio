// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package bind9_test

import (
	"context"
	"encoding/base64"
	"errors"
	"net/netip"
	"os"
	"strings"
	"testing"

	"github.com/Andrey3681/httpaas-orchestrator/internal/adapters/bind9"
	"github.com/Andrey3681/httpaas-orchestrator/internal/core/domain"
)

type mockExecutor struct {
	executedCommands [][]string
	updateScript     string
	failCommands     map[string]error
	stdoutByCommand  map[string]string
}

func (m *mockExecutor) Execute(ctx context.Context, command string, args ...string) ([]byte, []byte, error) {
	cmd := append([]string{command}, args...)
	m.executedCommands = append(m.executedCommands, cmd)
	if m.failCommands != nil {
		if err, ok := m.failCommands[command]; ok {
			return nil, []byte(err.Error()), err
		}
	}
	if command == "nsupdate" && len(args) > 0 {
		if data, err := os.ReadFile(args[len(args)-1]); err == nil {
			m.updateScript = string(data)
		}
	}
	if command == "ssh" && len(args) > 0 {
		last := args[len(args)-1]
		if strings.Contains(last, "nsupdate -v") {
			parts := strings.Split(last, "'")
			if len(parts) >= 2 {
				if raw, err := base64.StdEncoding.DecodeString(parts[1]); err == nil {
					m.updateScript = string(raw)
				}
			}
		}
	}
	if m.stdoutByCommand != nil {
		if out, ok := m.stdoutByCommand[command]; ok {
			return []byte(out), nil, nil
		}
	}
	return []byte("OK"), nil, nil
}

func TestDnsAdapter_AddRecord(t *testing.T) {
	exec := &mockExecutor{}
	adapter := bind9.NewDnsAdapter(exec, "192.168.10.10", "cloud.local", nil)

	rec, err := domain.NewARecord("paisajes", "cloud.local", netip.MustParseAddr("192.168.10.30"), 300)
	if err != nil {
		t.Fatalf("failed to create A record: %v", err)
	}

	err = adapter.AddRecord(context.Background(), *rec)
	if err != nil {
		t.Fatalf("unexpected error adding DNS record: %v", err)
	}

	if len(exec.executedCommands) == 0 {
		t.Fatalf("expected nsupdate command to be executed")
	}
	if !strings.Contains(exec.updateScript, "update add paisajes.cloud.local.") ||
		!strings.Contains(exec.updateScript, "A 192.168.10.30") {
		t.Errorf("el guion nsupdate no contiene el registro: %q", exec.updateScript)
	}
}

func TestDnsAdapter_AddRecordFallsBackToSSH(t *testing.T) {
	exec := &mockExecutor{
		failCommands: map[string]error{
			"nsupdate": errors.New("executable file not found"),
		},
	}
	adapter := bind9.NewDnsAdapter(exec, "192.168.176.10", "cloud.local", nil)
	adapter.ConfigureSSH(func() string { return "http" }, "/tmp/id_ed25519")

	rec, err := domain.NewARecord("paisajes", "cloud.local", netip.MustParseAddr("192.168.176.106"), 300)
	if err != nil {
		t.Fatalf("failed to create A record: %v", err)
	}

	if err := adapter.AddRecord(context.Background(), *rec); err != nil {
		t.Fatalf("unexpected error adding DNS record via SSH fallback: %v", err)
	}

	if !strings.Contains(exec.updateScript, "update add paisajes.cloud.local.") ||
		!strings.Contains(exec.updateScript, "A 192.168.176.106") {
		t.Errorf("el guion nsupdate por SSH no contiene el registro: %q", exec.updateScript)
	}
}

func TestDnsAdapter_ListRecordsFallsBackToSSH(t *testing.T) {
	exec := &mockExecutor{
		failCommands: map[string]error{
			"dig": errors.New("executable file not found"),
		},
		stdoutByCommand: map[string]string{
			"ssh": "ns.cloud.local.\t300\tIN\tA\t192.168.176.10\npaisajes.cloud.local.\t300\tIN\tA\t192.168.176.106\n",
		},
	}
	adapter := bind9.NewDnsAdapter(exec, "192.168.176.10", "cloud.local", nil)
	adapter.ConfigureSSH(func() string { return "http" }, "/tmp/id_ed25519")

	records, err := adapter.ListRecords(context.Background())
	if err != nil {
		t.Fatalf("unexpected error listing DNS records: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d: %+v", len(records), records)
	}
	if records[1].Name != "paisajes.cloud.local." || records[1].IP != "192.168.176.106" {
		t.Errorf("unexpected second record: %+v", records[1])
	}
}
