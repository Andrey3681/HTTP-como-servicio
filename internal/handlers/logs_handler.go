// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Andrey3681/httpaas-orchestrator/internal/logging"
)

const (
	defaultLogsLimit = 200
	maxLogsLimit     = 1000
)

// LogsHandler atiende los endpoints de consulta y streaming de logs.
type LogsHandler struct {
	broker *logging.Broker
}

// NewLogsHandler crea un nuevo LogsHandler respaldado por el broker indicado.
func NewLogsHandler(broker *logging.Broker) *LogsHandler {
	return &LogsHandler{broker: broker}
}

// HandleLogs responde con los eventos recientes del anillo de logs (/api/logs).
func (h *LogsHandler) HandleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Error(w, http.StatusMethodNotAllowed, "Método HTTP no permitido")
		return
	}

	limit := defaultLogsLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > maxLogsLimit {
			Error(w, http.StatusBadRequest, "El parámetro 'limit' debe ser un entero entre 1 y 1000")
			return
		}
		limit = parsed
	}

	entries := h.broker.Recent(limit)
	JSON(w, http.StatusOK, Response{
		Datos: entries,
		Total: len(entries),
	})
}

// HandleStream entrega eventos de log en tiempo real mediante Server-Sent Events (/api/logs/stream).
func (h *LogsHandler) HandleStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Error(w, http.StatusMethodNotAllowed, "Método HTTP no permitido")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		Error(w, http.StatusInternalServerError, "El servidor no soporta streaming de eventos")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch, cancel := h.broker.Subscribe()
	defer cancel()

	// Señal inicial para confirmar apertura del stream.
	fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case entry, ok := <-ch:
			if !ok {
				return
			}
			payload, err := json.Marshal(entry)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "id: %d\n", entry.ID)
			fmt.Fprintf(w, "data: %s\n\n", payload)
			flusher.Flush()
		}
	}
}
