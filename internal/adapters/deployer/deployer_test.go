// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package deployer_test

import (
	"bytes"
	"context"
	"net/netip"
	"testing"

	"github.com/Andrey3681/httpaas-orchestrator/internal/adapters/deployer"
)

type dummyExecutor struct{}

func (d *dummyExecutor) Execute(ctx context.Context, command string, args ...string) ([]byte, []byte, error) {
	return []byte("OK"), nil, nil
}

func TestApacheDeployer_DeployWebZip(t *testing.T) {
	dep := deployer.NewApacheDeployer(&dummyExecutor{})
	data := []byte("PK\x05\x06" + string(make([]byte, 18))) // ZIP vacío válido

	err := dep.DeployWebZip(context.Background(), netip.MustParseAddr("192.168.10.30"), bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("unexpected error deploying web zip: %v", err)
	}
}
