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
	// DeployWebZip transfiere y descomprime el contenido web en /var/www/html/ de la máquina virtual objetivo.
	DeployWebZip(ctx context.Context, targetIP netip.Addr, zipContent io.Reader, size int64) error
}
