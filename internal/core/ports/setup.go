// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package ports

import (
	"context"
	"time"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/domain"
)

// ProgressFunc reporta el avance de una operación larga. Un porcentaje negativo
// indica progreso indeterminado y el mensaje describe la actividad en curso.
type ProgressFunc func(percent int, message string)

// SetupProvisioner abstrae la preparación real de la infraestructura del laboratorio.
type SetupProvisioner interface {
	// Inspect detecta el estado actual del host sin modificarlo.
	Inspect(ctx context.Context, cfg domain.SetupConfig) (domain.EnvironmentStatus, error)

	// EnsureNetwork garantiza una red host-only y retorna su configuración real
	// (interfaz, IP del anfitrión, DNS y rango DHCP derivados de esa subred).
	EnsureNetwork(ctx context.Context, cfg domain.SetupConfig) (domain.NetworkConfig, error)

	// Network describe la red host-only existente sin modificarla.
	Network(ctx context.Context) (domain.NetworkConfig, error)

	// CreateVM registra una máquina virtual, duplica el VDI base y conecta el
	// disco. Retorna la ruta del disco creado.
	CreateVM(ctx context.Context, cfg domain.SetupConfig, vmName, netInterface, baseVDI string) (string, error)

	// ConvertToBaseVDI convierte la imagen raw en un VDI base reutilizable, una
	// sola vez. Si ya existe, no hace nada.
	ConvertToBaseVDI(ctx context.Context, cfg domain.SetupConfig, rawPath, baseVDI string, progress ProgressFunc) error

	// CreateSeedISO genera la ISO NoCloud con user-data y meta-data.
	CreateSeedISO(ctx context.Context, cfg domain.SetupConfig, vmName, userData, metaData string) (string, error)

	// StartSeedBoot conecta la ISO semilla y arranca la máquina en headless.
	StartSeedBoot(ctx context.Context, cfg domain.SetupConfig, vmName, seedISO string) error

	// WaitGuestReady espera a que Guest Additions y la red estén disponibles,
	// reportando hitos reales del invitado mientras tanto.
	WaitGuestReady(ctx context.Context, vmName string, timeout time.Duration, progress ...ProgressFunc) error

	// RunGuestScript copia un script local al invitado, lo ejecuta como root con
	// los argumentos indicados y retorna su salida.
	RunGuestScript(ctx context.Context, cfg domain.SetupConfig, vmName, localScript string, args ...string) (string, error)

	// ReserveDHCPAddress fija una dirección IP para la MAC de la VM indicada.
	ReserveDHCPAddress(ctx context.Context, cfg domain.SetupConfig, vmName, ip string) error

	// MarkBaseMultiattach apaga la plantilla, marca su disco como multiconexión y
	// retorna la ruta del disco base.
	MarkBaseMultiattach(ctx context.Context, cfg domain.SetupConfig, vmName string) (string, error)

	// ResetEnvironment elimina las VMs base y sus semillas para reiniciar la
	// preparación. No borra la imagen descargada ni la red.
	ResetEnvironment(ctx context.Context, cfg domain.SetupConfig) error
}

// NetworkInfoProvider expone la red host-only ya detectada.
type NetworkInfoProvider interface {
	Network() domain.NetworkConfig
}

// ImageStore localiza o descarga la imagen ISO del sistema invitado.
type ImageStore interface {
	// Locate busca una ISO compatible en el directorio indicado.
	Locate(ctx context.Context, dir string) (string, bool)

	// Download descarga la ISO desde url y retorna la ruta final.
	Download(ctx context.Context, url, dir string, progress ProgressFunc) (string, error)
}
