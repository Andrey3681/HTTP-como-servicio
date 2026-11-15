// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package ports

import (
	"context"
	"net/netip"
)

// InstanceOperator ejecuta operaciones dentro de una instancia ya aprovisionada.
type InstanceOperator interface {
	// RestartWebService reinicia Apache en la instancia y verifica que quede activo.
	RestartWebService(ctx context.Context, hostname string, ip netip.Addr) error
}
