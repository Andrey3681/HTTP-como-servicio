// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package ports

import "context"

// HostDNS configura el resolver del anfitrión para la zona del laboratorio.
// Requiere privilegios de administrador (pkexec o ejecución como root).
type HostDNS interface {
	// Configure aplica la configuración persistente para la interfaz e IP del
	// DNS del laboratorio y la activa de inmediato.
	Configure(ctx context.Context, iface, dnsIP string) error
}
