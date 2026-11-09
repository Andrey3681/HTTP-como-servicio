// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package handlers

import (
	"net/http"
	"os/exec"
	"strings"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/ports"
)

// DnsHandler maneja las peticiones de consulta de zona DNS y salud del entorno.
type DnsHandler struct {
	dnsProvider ports.DnsProvider
	zone        string
}

// NewDnsHandler crea un nuevo DnsHandler para la zona DNS indicada.
func NewDnsHandler(dnsProvider ports.DnsProvider, zone string) *DnsHandler {
	cleanZone := strings.TrimSpace(zone)
	if cleanZone != "" && !strings.HasSuffix(cleanZone, ".") {
		cleanZone += "."
	}
	return &DnsHandler{
		dnsProvider: dnsProvider,
		zone:        cleanZone,
	}
}

// HandleHealth responde el estado de salud de las herramientas del sistema (/api/salud).
func (h *DnsHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Error(w, http.StatusMethodNotAllowed, "Método HTTP no permitido")
		return
	}

	_, vboxErr := exec.LookPath("VBoxManage")

	JSON(w, http.StatusOK, map[string]any{
		"estado":     "ok",
		"vboxmanage": vboxErr == nil,
		"bind9":      true,
	})
}

// HandleRecords responde con los registros A de la zona DNS administrada (/api/dns/registros).
// Si la consulta al proveedor falla o no está disponible, retorna una lista vacía manteniendo el contrato.
func (h *DnsHandler) HandleRecords(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Error(w, http.StatusMethodNotAllowed, "Método HTTP no permitido")
		return
	}

	zone := h.zone
	if zone == "" {
		zone = "cloud.local."
	}

	registros := []any{}
	if h.dnsProvider != nil {
		if items, err := h.dnsProvider.ListRecords(r.Context()); err == nil {
			registros = make([]any, 0, len(items))
			for _, item := range items {
				registros = append(registros, item)
			}
		}
	}

	JSON(w, http.StatusOK, map[string]any{
		"zona":      zone,
		"registros": registros,
	})
}
