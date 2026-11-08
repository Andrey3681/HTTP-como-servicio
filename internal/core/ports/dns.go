// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package ports

import (
	"context"
	"net/netip"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/domain"
)

// DnsProvider define la gestión dinámica de registros en el servidor DNS Bind9.
type DnsProvider interface {
	// AddRecord inserta un registro tipo A en la zona DNS.
	AddRecord(ctx context.Context, record domain.DnsRecord) error

	// RemoveRecord elimina un registro tipo A previamente creado.
	RemoveRecord(ctx context.Context, hostname string) error

	// Resolve verifica la resolución directa del nombre de host consultando al DNS.
	Resolve(ctx context.Context, hostname string) (netip.Addr, error)
}
