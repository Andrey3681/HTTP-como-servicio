// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/domain"
	"github.com/Andrey3681/httpaas-orchestrator/internal/services"
)

// SetupHandler expone el configuración inicial.
type SetupHandler struct {
	setup *services.SetupService
	done  <-chan struct{}
}

// NewSetupHandler crea el manejador de la preparación sobre el servicio indicado.
// El canal opcional done permite cerrar los streams SSE durante el apagado.
func NewSetupHandler(setup *services.SetupService, done ...<-chan struct{}) *SetupHandler {
	h := &SetupHandler{setup: setup}
	if len(done) > 0 {
		h.done = done[0]
	}
	return h
}

// HandleEstado responde la instantánea actual de la preparación (/api/setup/estado).
func (h *SetupHandler) HandleEstado(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Error(w, http.StatusMethodNotAllowed, "Método HTTP no permitido")
		return
	}
	JSON(w, http.StatusOK, h.setup.Estado())
}

// HandleConfigurar valida y guarda los datos del formulario (/api/setup/configuracion).
func (h *SetupHandler) HandleConfigurar(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Error(w, http.StatusMethodNotAllowed, "Método HTTP no permitido")
		return
	}

	var input domain.SetupInput
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&input); err != nil {
		Error(w, http.StatusBadRequest, "Datos de configuración inválidos")
		return
	}
	if err := h.setup.Configurar(input); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	JSON(w, http.StatusOK, Response{Mensaje: "Configuración de la preparación guardada"})
}

// HandleIniciar arranca la configuración inicial (/api/setup/iniciar).
func (h *SetupHandler) HandleIniciar(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Error(w, http.StatusMethodNotAllowed, "Método HTTP no permitido")
		return
	}

	err := h.setup.Iniciar(r.Context())
	switch {
	case err == nil:
		JSON(w, http.StatusAccepted, Response{Mensaje: "Configuración inicial iniciada"})
	case errors.Is(err, services.ErrSetupEjecutando):
		Error(w, http.StatusConflict, err.Error())
	case errors.Is(err, services.ErrSetupCompletado):
		Error(w, http.StatusConflict, err.Error())
	case errors.Is(err, services.ErrSetupSinConfigurar):
		Error(w, http.StatusConflict, err.Error())
	default:
		Error(w, http.StatusInternalServerError, "No se pudo iniciar la configuración inicial")
	}
}

// HandleReiniciar elimina las VMs base y reinicia la preparación (/api/setup/reiniciar).
func (h *SetupHandler) HandleReiniciar(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Error(w, http.StatusMethodNotAllowed, "Método HTTP no permitido")
		return
	}
	err := h.setup.Reiniciar(r.Context())
	switch {
	case err == nil:
		JSON(w, http.StatusOK, Response{Mensaje: "Preparación reiniciada"})
	case errors.Is(err, services.ErrSetupEjecutando):
		Error(w, http.StatusConflict, err.Error())
	default:
		Error(w, http.StatusInternalServerError, "No se pudo reiniciar la preparación: "+err.Error())
	}
}

// HandleStream entrega el estado de la preparación en tiempo real (/api/setup/stream).
func (h *SetupHandler) HandleStream(w http.ResponseWriter, r *http.Request) {
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

	if err := writeSetupEvent(w, h.setup.Estado()); err != nil {
		return
	}
	flusher.Flush()

	updates, cancel := h.setup.Subscribe()
	defer cancel()

	keepAlive := time.NewTicker(20 * time.Second)
	defer keepAlive.Stop()

	for {
		select {
		case <-h.done:
			return
		case <-r.Context().Done():
			return
		case <-keepAlive.C:
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case state, ok := <-updates:
			if !ok {
				return
			}
			if err := writeSetupEvent(w, state); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func writeSetupEvent(w http.ResponseWriter, state domain.SetupState) error {
	payload, err := json.Marshal(state)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", payload)
	return err
}
