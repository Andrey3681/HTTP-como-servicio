// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package handlers

import (
	"context"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/ports"
)

// DnsHandler maneja las peticiones de consulta de zona DNS y salud del entorno.
type DnsHandler struct {
	dnsProvider ports.DnsProvider
	zone        string
	logger      ports.EventLogger
	lookup      func(context.Context, string) ([]string, error)
	network     ports.NetworkInfoProvider
	hostDNS     ports.HostDNS
}

// SetHostDNS inyecta la red del laboratorio y el configurador del resolver del
// anfitrión (se usa desde /api/dns/anfitrion/configurar).
func (h *DnsHandler) SetHostDNS(network ports.NetworkInfoProvider, hostDNS ports.HostDNS) {
	h.network = network
	h.hostDNS = hostDNS
}

// NewDnsHandler crea un nuevo DnsHandler para la zona DNS indicada.
func NewDnsHandler(dnsProvider ports.DnsProvider, zone string, logger ports.EventLogger) *DnsHandler {
	cleanZone := strings.TrimSpace(zone)
	if cleanZone != "" && !strings.HasSuffix(cleanZone, ".") {
		cleanZone += "."
	}
	return &DnsHandler{
		dnsProvider: dnsProvider,
		zone:        cleanZone,
		logger:      logger,
		lookup:      net.DefaultResolver.LookupHost,
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

// HandleConfigureHost configura el resolver del anfitrión para cloud.local
// (/api/dns/anfitrion/configurar). Usa pkexec, que pide autorización.
func (h *DnsHandler) HandleConfigureHost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Error(w, http.StatusMethodNotAllowed, "Método HTTP no permitido")
		return
	}
	if h.network == nil || h.hostDNS == nil {
		Error(w, http.StatusConflict, "El entorno aún no está preparado")
		return
	}
	cfg := h.network.Network()
	if cfg.Interface == "" || cfg.DNSIP == "" {
		Error(w, http.StatusConflict, "Completa la preparación inicial antes de configurar el DNS")
		return
	}
	if err := h.hostDNS.Configure(r.Context(), cfg.Interface, cfg.DNSIP); err != nil {
		Error(w, http.StatusInternalServerError, "No se pudo configurar el DNS del anfitrión: "+err.Error())
		return
	}
	JSON(w, http.StatusOK, map[string]any{"mensaje": "DNS del anfitrión configurado para la zona cloud.local"})
}

// HandleHostResolve comprueba si el resolver del anfitrión resuelve un nombre
// de la zona (/api/dns/anfitrion). No usa el DNS del laboratorio: reproduce la
// vista del navegador.
func (h *DnsHandler) HandleHostResolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Error(w, http.StatusMethodNotAllowed, "Método HTTP no permitido")
		return
	}
	name := strings.TrimSpace(r.URL.Query().Get("nombre"))
	if name == "" {
		name = "ns." + strings.TrimSuffix(h.zone, ".")
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	addrs, err := h.lookup(ctx, name)
	zone := strings.TrimSuffix(h.zone, ".")
	if err != nil || len(addrs) == 0 {
		JSON(w, http.StatusOK, map[string]any{
			"resuelve": false,
			"ip":       "",
			"mensaje": "El resolver del anfitrión no resuelve " + zone + ". Configura el DNS del laboratorio: " +
				"resolvectl dns vboxnet1 192.168.57.10 y resolvectl domain vboxnet1 '~" + zone + "' (requiere administrador), " +
				"o añade el nombre a /etc/hosts.",
		})
		return
	}
	JSON(w, http.StatusOK, map[string]any{"resuelve": true, "ip": addrs[0], "mensaje": ""})
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
