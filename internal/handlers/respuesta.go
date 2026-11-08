// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

// Package handlers contiene los adaptadores primarios que exponen la API REST y gestionan las peticiones HTTP.
package handlers

import (
	"encoding/json"
	"net/http"
)

// Response representa el formato estándar de respuesta exitosa.
type Response struct {
	Mensaje string `json:"mensaje,omitempty"`
	Datos   any    `json:"datos,omitempty"`
	Total   int    `json:"total,omitempty"`
}

// ErrorResponse representa el formato estándar de respuesta ante fallas.
type ErrorResponse struct {
	Error  string `json:"error"`
	Codigo int    `json:"codigo"`
}

// JSON escribe una respuesta JSON exitosa con el código de estado indicado.
func JSON(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(data)
}

// Error escribe una respuesta JSON de error con el código HTTP correspondiente.
func Error(w http.ResponseWriter, statusCode int, message string) {
	JSON(w, statusCode, ErrorResponse{
		Error:  message,
		Codigo: statusCode,
	})
}
