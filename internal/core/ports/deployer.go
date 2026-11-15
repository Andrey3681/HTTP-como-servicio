// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package ports

import (
	"context"
	"io"
	"net/netip"
)

// Deployer define la transferencia y despliegue del paquete web en la instancia Apache.
type Deployer interface {
	// DeployWebZip transfiere y descomprime el contenido web en
	// /var/www/vhosts/<hostname>/public y publica su virtual host.
	DeployWebZip(ctx context.Context, hostname string, targetIP netip.Addr, zipContent io.Reader, size int64) error
}
