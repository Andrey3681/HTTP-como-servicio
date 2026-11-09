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

var allowedLogLevels = map[string]bool{
	string(logging.LevelDebug): true,
	string(logging.LevelInfo):  true,
	string(logging.LevelWarn):  true,
	string(logging.LevelError): true,
}

// LogsHandler atiende los endpoints de consulta, limpieza y streaming de logs.
type LogsHandler struct {
	broker *logging.Broker
}

// NewLogsHandler crea un nuevo LogsHandler respaldado por el broker indicado.
func NewLogsHandler(broker *logging.Broker) *LogsHandler {
	return &LogsHandler{broker: broker}
}

// HandleLogs enruta operaciones de colección de logs (/api/logs): listado (GET) y limpieza (DELETE).
func (h *LogsHandler) HandleLogs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.list(w, r)
	case http.MethodDelete:
		h.clear(w, r)
	default:
		Error(w, http.StatusMethodNotAllowed, "Método HTTP no permitido")
	}
}

func (h *LogsHandler) list(w http.ResponseWriter, r *http.Request) {
	opts := logging.QueryOptions{Limit: defaultLogsLimit}
	query := r.URL.Query()

	if raw := query.Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > maxLogsLimit {
			Error(w, http.StatusBadRequest, "El parámetro 'limit' debe ser un entero entre 1 y 1000")
			return
		}
		opts.Limit = parsed
	}
	if nivel := query.Get("nivel"); nivel != "" {
		if !allowedLogLevels[nivel] {
			Error(w, http.StatusBadRequest, "El parámetro 'nivel' no es válido")
			return
		}
		opts.Level = nivel
	}
	if componente := query.Get("componente"); componente != "" {
		opts.Component = componente
	}
	if raw := query.Get("since"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed < 0 {
			Error(w, http.StatusBadRequest, "El parámetro 'since' debe ser un entero mayor o igual a 0")
			return
		}
		opts.Since = parsed
	}

	entries := h.broker.Query(opts)
	JSON(w, http.StatusOK, Response{
		Datos: entries,
		Total: len(entries),
	})
}

func (h *LogsHandler) clear(w http.ResponseWriter, r *http.Request) {
	h.broker.Clear()
	JSON(w, http.StatusOK, Response{
		Mensaje: "Registros de log limpiados",
	})
}

// HandleStream entrega eventos de log en tiempo real mediante Server-Sent Events (/api/logs/stream).
func (h *LogsHandler) HandleStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Error(w, http.StatusMethodNotAllowed, "Método HTTP no permitido")
		return
	}

	since, err := parseSince(r)
	if err != nil {
		Error(w, http.StatusBadRequest, "El parámetro 'since' no es válido")
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

	fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	for _, entry := range h.broker.Since(since) {
		if !writeSSEEvent(w, flusher, entry) {
			return
		}
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case entry, ok := <-ch:
			if !ok {
				return
			}
			if !writeSSEEvent(w, flusher, entry) {
				return
			}
		}
	}
}

func parseSince(r *http.Request) (int64, error) {
	if raw := r.URL.Query().Get("since"); raw != "" {
		return strconv.ParseInt(raw, 10, 64)
	}
	if raw := r.Header.Get("Last-Event-ID"); raw != "" {
		return strconv.ParseInt(raw, 10, 64)
	}
	return 0, nil
}

func writeSSEEvent(w http.ResponseWriter, flusher http.Flusher, entry logging.Entry) bool {
	payload, err := json.Marshal(entry)
	if err != nil {
		return true
	}
	if _, err := fmt.Fprintf(w, "id: %d\n", entry.ID); err != nil {
		return false
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
		return false
	}
	flusher.Flush()
	return true
}
