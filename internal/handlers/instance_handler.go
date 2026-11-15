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

// HandleInstance enruta las peticiones individuales (/api/instancias/{id}) y
// sus subrutas de operación.
func (h *InstanceHandler) HandleInstance(w http.ResponseWriter, r *http.Request) {
	prefix := "/api/instancias/"
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/")
	if rest == "" {
		Error(w, http.StatusBadRequest, "Identificador de instancia no especificado")
		return
	}

	parts := strings.Split(rest, "/")
	id := parts[0]
	action := ""
	if len(parts) > 1 {
		action = strings.Join(parts[1:], "/")
	}

	switch action {
	case "":
		switch r.Method {
		case http.MethodGet:
			h.get(w, r, id)
		case http.MethodDelete:
			h.delete(w, r, id)
		default:
			Error(w, http.StatusMethodNotAllowed, "Método HTTP no permitido")
		}
	case "acceso":
		if r.Method != http.MethodGet {
			Error(w, http.StatusMethodNotAllowed, "Método HTTP no permitido")
			return
		}
		h.access(w, r, id)
	case "servicios/reiniciar":
		if r.Method != http.MethodPost {
			Error(w, http.StatusMethodNotAllowed, "Método HTTP no permitido")
			return
		}
		h.restartServices(w, r, id)
	case "despliegue":
		if r.Method != http.MethodPost {
			Error(w, http.StatusMethodNotAllowed, "Método HTTP no permitido")
			return
		}
		h.redeploy(w, r, id)
	default:
		Error(w, http.StatusNotFound, "Recurso no encontrado")
	}
}

// access responde los datos y el comando SSH de la instancia.
func (h *InstanceHandler) access(w http.ResponseWriter, r *http.Request, id string) {
	data, err := h.instanceService.GetAccess(r.Context(), id)
	if err != nil {
		h.operationError(w, id, "acceso", err)
		return
	}
	JSON(w, http.StatusOK, Response{Datos: data})
}

// restartServices reinicia Apache en la instancia.
func (h *InstanceHandler) restartServices(w http.ResponseWriter, r *http.Request, id string) {
	inst, err := h.instanceService.RestartServices(r.Context(), id)
	if err != nil {
		h.operationError(w, id, "reiniciar servicios", err)
		return
	}
	JSON(w, http.StatusOK, Response{
		Mensaje: "Servicios de la instancia reiniciados",
		Datos: map[string]any{
			"id":        inst.ID,
			"estado":    inst.Status,
			"resultado": "apache2 activo",
		},
	})
}

// redeploy reemplaza el contenido web con un nuevo paquete .zip.
func (h *InstanceHandler) redeploy(w http.ResponseWriter, r *http.Request, id string) {
	if err := r.ParseMultipartForm(55 * 1024 * 1024); err != nil {
		Error(w, http.StatusBadRequest, "No se pudo procesar la solicitud multipart")
		return
	}
	file, header, err := r.FormFile("archivo")
	if err != nil {
		Error(w, http.StatusBadRequest, "El archivo comprimido 'archivo' es obligatorio")
		return
	}
	defer func() { _ = file.Close() }()

	if !strings.HasSuffix(strings.ToLower(header.Filename), ".zip") {
		Error(w, http.StatusBadRequest, "El archivo debe tener extensión .zip")
		return
	}
	buf, err := io.ReadAll(file)
	if err != nil {
		logHandlerError(h.logger, "http", "Fallo al leer paquete de despliegue", map[string]any{"id": id, "error": err.Error()})
		Error(w, http.StatusInternalServerError, "Error interno al procesar el archivo recibido")
		return
	}

	inst, err := h.instanceService.Redeploy(r.Context(), id, bytes.NewReader(buf), int64(len(buf)))
	if err != nil {
		h.operationError(w, id, "actualizar despliegue", err)
		return
	}
	JSON(w, http.StatusOK, Response{Mensaje: "Despliegue actualizado correctamente", Datos: inst})
}

// operationError traduce errores de dominio a respuestas HTTP.
func (h *InstanceHandler) operationError(w http.ResponseWriter, id, action string, err error) {
	switch {
	case errors.Is(err, domain.ErrInstanceNotFound):
		Error(w, http.StatusNotFound, "La instancia solicitada no existe")
	case errors.Is(err, domain.ErrOperationNotAllowed):
		Error(w, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrInvalidWebPackage), errors.Is(err, domain.ErrZipSlipDetected):
		Error(w, http.StatusBadRequest, err.Error())
	default:
		logHandlerError(h.logger, "http", "Fallo en "+action, map[string]any{"id": id, "error": err.Error()})
		Error(w, http.StatusInternalServerError, "Error interno al "+action)
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
	defer func() { _ = file.Close() }()

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
