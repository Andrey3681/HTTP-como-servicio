// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package ports

import (
	"context"
)

// Executor abstrae la invocación de comandos y procesos en el sistema operativo anfitrión.
type Executor interface {
	// Execute ejecuta un comando del sistema con sus argumentos y retorna la salida combinada o error.
	Execute(ctx context.Context, command string, args ...string) (stdout []byte, stderr []byte, err error)
}
