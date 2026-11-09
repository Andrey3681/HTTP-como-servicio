// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package ports

// EventLogger define la emisión de eventos estructurados de observabilidad.
type EventLogger interface {
	// Append registra un evento con nivel, componente, mensaje y campos opcionales.
	Append(level string, component string, message string, fields map[string]any)
}
