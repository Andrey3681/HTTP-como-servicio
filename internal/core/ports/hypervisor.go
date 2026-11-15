// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

// Package ports define los contratos de interfaz que comunican el núcleo de la aplicación con la infraestructura externa.
package ports

import (
	"context"
	"net/netip"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/domain"
)

// Hypervisor define las operaciones requeridas sobre el hipervisor VirtualBox.
type Hypervisor interface {
	// CreateInstanceFromBase aprovisiona una nueva VM enlazando el disco multiconexión base.
	CreateInstanceFromBase(ctx context.Context, name string, baseDiskPath string) (*domain.Instance, error)

	// Start inicia la máquina virtual en modo desatendido (headless).
	Start(ctx context.Context, vmName string) error

	// GetIPAddress obtiene la IP de la VM: consulta la concesión DHCP por MAC y,
	// si no está disponible, recurre a Guest Additions como último recurso.
	GetIPAddress(ctx context.Context, vmName string) (netip.Addr, error)

	// Stop apaga la máquina virtual.
	Stop(ctx context.Context, vmName string) error

	// Destroy desmantela la VM, desasocia el disco diferencial y elimina los archivos físicos.
	Destroy(ctx context.Context, vmName string) error
}

// PowerStateInspector permite consultar qué máquinas virtuales se encuentran
// realmente en ejecución en el hipervisor para sincronizar el inventario.
type PowerStateInspector interface {
	// RunningVMs retorna el conjunto de nombres de máquinas virtuales en ejecución.
	RunningVMs(ctx context.Context) (map[string]bool, error)
}
