// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package bind9_test

import (
	"context"
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
}

func (m *mockExecutor) Execute(ctx context.Context, command string, args ...string) ([]byte, []byte, error) {
	cmd := append([]string{command}, args...)
	m.executedCommands = append(m.executedCommands, cmd)
	if command == "nsupdate" && len(args) > 0 {
		if data, err := os.ReadFile(args[len(args)-1]); err == nil {
			m.updateScript = string(data)
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
