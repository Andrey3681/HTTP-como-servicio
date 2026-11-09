// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package vbox_test

import (
	"context"
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

type mockExecutor struct {
	commandsRun [][]string
}

func (m *mockExecutor) Execute(ctx context.Context, command string, args ...string) ([]byte, []byte, error) {
	full := append([]string{command}, args...)
	m.commandsRun = append(m.commandsRun, full)
	return []byte("OK"), nil, nil
}

func TestHypervisorAdapter_CreateInstanceFromBase(t *testing.T) {
	mockExec := &mockExecutor{}
	adapter := vbox.NewHypervisorAdapter(mockExec, "cloud.local", nil)

	inst, err := adapter.CreateInstanceFromBase(context.Background(), "paisajes", "ApacheServer.vdi")
	if err != nil {
		t.Fatalf("unexpected error creating instance: %v", err)
	}

	if inst.Name != "paisajes" {
		t.Errorf("expected name 'paisajes', got '%s'", inst.Name)
	}

	// Debe haber ejecutado createvm, modifyvm, storagectl y storageattach
	if len(mockExec.commandsRun) != 4 {
		t.Errorf("expected 4 commands executed, got %d", len(mockExec.commandsRun))
	}
}
