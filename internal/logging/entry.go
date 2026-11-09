// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

// Package logging provee el modelo de eventos y el broker de logs en memoria para HTTPaaS.
package logging

import "time"

// Level representa el nivel de severidad de un evento de log.
type Level string

// Niveles de severidad soportados por el contrato de logs.
const (
	LevelDebug Level = "DEBUG"
	LevelInfo  Level = "INFO"
	LevelWarn  Level = "WARN"
	LevelError Level = "ERROR"
)

// Entry representa un evento de log estructurado.
type Entry struct {
	ID         int64          `json:"id"`
	TS         time.Time      `json:"ts"`
	Nivel      Level          `json:"nivel"`
	Componente string         `json:"componente"`
	Mensaje    string         `json:"mensaje"`
	Campos     map[string]any `json:"campos,omitempty"`
}
