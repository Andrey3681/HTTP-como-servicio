// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package hostdns_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Andrey3681/httpaas-orchestrator/internal/adapters/hostdns"
)

type stubExecutor struct {
	calls [][]string
}

func (s *stubExecutor) Execute(_ context.Context, command string, args ...string) ([]byte, []byte, error) {
	s.calls = append(s.calls, append([]string{command}, args...))
	return []byte("OK"), nil, nil
}

func TestAdapter_Configure(t *testing.T) {
	exec := &stubExecutor{}
	adapter := hostdns.New(exec)

	if err := adapter.Configure(context.Background(), "VirtualBox Host-Only Ethernet Adapter #2", "192.168.176.10"); err != nil {
		t.Fatalf("unexpected error configuring host DNS: %v", err)
	}
	if len(exec.calls) == 0 {
		t.Fatal("expected at least one command to be executed")
	}
	joined := strings.Join(exec.calls[0], " ")
	if !strings.Contains(joined, "192.168.176.10") && !strings.Contains(joined, "-EncodedCommand") {
		t.Errorf("expected DNS configuration command, got: %s", joined)
	}
}
