// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package handlers

import "github.com/Andrey3681/httpaas-orchestrator/internal/core/ports"

func logHandlerError(logger ports.EventLogger, component, message string, fields map[string]any) {
	if logger == nil {
		return
	}
	logger.Append("ERROR", component, message, fields)
}
