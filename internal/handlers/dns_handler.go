// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package handlers

import (
	"net/http"
	"os/exec"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/ports"
)

// DnsHandler maneja las peticiones de consulta de zona DNS y salud del entorno.
type DnsHandler struct {
	dnsProvider ports.DnsProvider
}

// NewDnsHandler crea un nuevo DnsHandler.
func NewDnsHandler(dnsProvider ports.DnsProvider) *DnsHandler {
	return &DnsHandler{
		dnsProvider: dnsProvider,
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
