// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package handlers

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/domain"
	"github.com/Andrey3681/httpaas-orchestrator/internal/core/ports"
	"github.com/Andrey3681/httpaas-orchestrator/internal/services"
)

// InstanceHandler atiende los endpoints de gestión de instancias.
type InstanceHandler struct {
	orchestrator    *services.Orchestrator
	instanceService *services.InstanceService
	logger          ports.EventLogger
}

// NewInstanceHandler crea un nuevo InstanceHandler con los servicios requeridos.
func NewInstanceHandler(orchestrator *services.Orchestrator, instanceService *services.InstanceService, logger ports.EventLogger) *InstanceHandler {
	return &InstanceHandler{
		orchestrator:    orchestrator,
		instanceService: instanceService,
		logger:          logger,
	}
}

// HandleInstances enruta las peticiones de colección (/api/instancias).
func (h *InstanceHandler) HandleInstances(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.list(w, r)
	case http.MethodPost:
		h.provision(w, r)
	default:
		Error(w, http.StatusMethodNotAllowed, "Método HTTP no permitido")
	}
}

// HandleInstance enruta las peticiones individuales (/api/instancias/{id}).
func (h *InstanceHandler) HandleInstance(w http.ResponseWriter, r *http.Request) {
	// Extraer id de la ruta: /api/instancias/<id>
	prefix := "/api/instancias/"
	id := strings.TrimPrefix(r.URL.Path, prefix)
	id = strings.Trim(id, "/")

	if id == "" {
		Error(w, http.StatusBadRequest, "Identificador de instancia no especificado")
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.get(w, r, id)
	case http.MethodDelete:
		h.delete(w, r, id)
	default:
		Error(w, http.StatusMethodNotAllowed, "Método HTTP no permitido")
	}
}

func (h *InstanceHandler) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.instanceService.ListInstances(r.Context())
	if err != nil {
		logHandlerError(h.logger, "http", "Fallo al consultar listado de instancias", map[string]any{"error": err.Error()})
		Error(w, http.StatusInternalServerError, "Error interno al consultar las instancias")
		return
	}

	JSON(w, http.StatusOK, Response{
		Datos: items,
		Total: len(items),
	})
}

func (h *InstanceHandler) get(w http.ResponseWriter, r *http.Request, id string) {
	inst, err := h.instanceService.GetInstance(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrInstanceNotFound) {
			Error(w, http.StatusNotFound, "La instancia solicitada no existe")
			return
		}
		logHandlerError(h.logger, "http", "Fallo al consultar la instancia", map[string]any{"id": id, "error": err.Error()})
		Error(w, http.StatusInternalServerError, "Error interno al consultar la instancia")
		return
	}

	JSON(w, http.StatusOK, Response{
		Datos: inst,
	})
}

func (h *InstanceHandler) provision(w http.ResponseWriter, r *http.Request) {
	// Límite de carga: 55 MB en memoria/disco temporal
	err := r.ParseMultipartForm(55 * 1024 * 1024)
	if err != nil {
		Error(w, http.StatusBadRequest, "No se pudo procesar la solicitud multipart")
		return
	}

	nombre := strings.TrimSpace(r.FormValue("nombre"))
	if nombre == "" {
		Error(w, http.StatusBadRequest, "El campo 'nombre' es obligatorio")
		return
	}

	file, header, err := r.FormFile("archivo")
	if err != nil {
		Error(w, http.StatusBadRequest, "El archivo comprimido 'archivo' es obligatorio")
		return
	}
	defer file.Close()

	if !strings.HasSuffix(strings.ToLower(header.Filename), ".zip") {
		Error(w, http.StatusBadRequest, "El archivo debe tener extensión .zip")
		return
	}

	// Leer a buffer en memoria para permitir ReaderAt y re-lectura
	buf, err := io.ReadAll(file)
	if err != nil {
		logHandlerError(h.logger, "http", "Fallo al leer archivo de aprovisionamiento", map[string]any{"error": err.Error()})
		Error(w, http.StatusInternalServerError, "Error interno al procesar el archivo recibido")
		return
	}

	req := services.ProvisionRequest{
		Hostname:   nombre,
		ZipContent: bytes.NewReader(buf),
		ZipSize:    int64(len(buf)),
		ZipStream:  bytes.NewReader(buf),
	}

	inst, err := h.orchestrator.Provision(r.Context(), req)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidHostname) ||
			errors.Is(err, domain.ErrHostAlreadyExists) ||
			errors.Is(err, domain.ErrInvalidWebPackage) ||
			errors.Is(err, domain.ErrZipSlipDetected) {
			Error(w, http.StatusBadRequest, err.Error())
			return
		}
		logHandlerError(h.logger, "http", "Fallo en aprovisionamiento de instancia", map[string]any{"nombre": nombre, "error": err.Error()})
		Error(w, http.StatusInternalServerError, "Error interno al aprovisionar la instancia")
		return
	}

	JSON(w, http.StatusCreated, Response{
		Mensaje: "Instancia aprovisionada con éxito",
		Datos:   inst,
	})
}

func (h *InstanceHandler) delete(w http.ResponseWriter, r *http.Request, id string) {
	err := h.instanceService.DeleteInstance(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrInstanceNotFound) {
			Error(w, http.StatusNotFound, "La instancia solicitada no existe")
			return
		}
		logHandlerError(h.logger, "http", "Fallo al eliminar instancia", map[string]any{"id": id, "error": err.Error()})
		Error(w, http.StatusInternalServerError, "Error interno al eliminar la instancia")
		return
	}

	JSON(w, http.StatusOK, Response{
		Mensaje: "Instancia eliminada exitosamente",
	})
}
