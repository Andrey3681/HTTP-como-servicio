// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package handlers

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/netip"
	"os/exec"
	"strings"
	"time"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/domain"
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
	hypervisor  ports.Hypervisor
	dnsVMName   string
}

// SetHostDNS inyecta la red del laboratorio y el configurador del resolver del
// anfitrión (se usa desde /api/dns/anfitrion/configurar).
func (h *DnsHandler) SetHostDNS(network ports.NetworkInfoProvider, hostDNS ports.HostDNS) {
	h.network = network
	h.hostDNS = hostDNS
}

// SetHypervisor inyecta el hipervisor y el nombre de la máquina virtual DNS
// para detectar si ns está encendida y permitir arrancarla bajo demanda.
func (h *DnsHandler) SetHypervisor(hypervisor ports.Hypervisor, dnsVMName string) {
	h.hypervisor = hypervisor
	if strings.TrimSpace(dnsVMName) == "" {
		dnsVMName = "ns"
	}
	h.dnsVMName = dnsVMName
}

func (h *DnsHandler) nsName() string {
	if h.dnsVMName != "" {
		return h.dnsVMName
	}
	return "ns"
}

func (h *DnsHandler) inspectNSRunning(ctx context.Context) (bool, bool) {
	inspector, ok := h.hypervisor.(ports.PowerStateInspector)
	if !ok || inspector == nil {
		return true, false
	}
	running, err := inspector.RunningVMs(ctx)
	if err != nil {
		return false, false
	}
	return running[h.nsName()], true
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
	vboxOk := vboxErr == nil
	bindOk := true
	nsActiva := true
	if running, inspected := h.inspectNSRunning(r.Context()); inspected {
		nsActiva = running
		bindOk = running
	}
	estado := "ok"
	if !vboxOk || !bindOk {
		estado = "degradado"
	}

	JSON(w, http.StatusOK, map[string]any{
		"estado":     estado,
		"vboxmanage": vboxOk,
		"bind9":      bindOk,
		"ns_activa":  nsActiva,
		"ns_vm":      h.nsName(),
	})
}

// HandleStartDNS enciende la máquina virtual del servidor DNS (/api/dns/encender).
func (h *DnsHandler) HandleStartDNS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Error(w, http.StatusMethodNotAllowed, "Método HTTP no permitido")
		return
	}
	if h.hypervisor == nil {
		Error(w, http.StatusConflict, "El hipervisor no está disponible")
		return
	}
	vmName := h.nsName()
	if running, inspected := h.inspectNSRunning(r.Context()); !inspected || !running {
		if err := h.hypervisor.Start(r.Context(), vmName); err != nil {
			logHandlerError(h.logger, "dns", "Fallo al encender máquina virtual DNS", map[string]any{"vm": vmName, "error": err.Error()})
			Error(w, http.StatusInternalServerError, "No se pudo encender la máquina DNS ("+vmName+"): "+err.Error())
			return
		}
	}
	if h.logger != nil {
		h.logger.Append("INFO", "dns", "Máquina virtual DNS encendida", map[string]any{"vm": vmName})
	}
	JSON(w, http.StatusOK, map[string]any{
		"mensaje":   "Servidor DNS (" + vmName + ") encendido correctamente",
		"ns_activa": true,
		"ns_vm":     vmName,
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

	nsActiva := true
	if running, inspected := h.inspectNSRunning(r.Context()); inspected {
		nsActiva = running
	}

	addrs, err := h.lookup(ctx, name)
	response := map[string]any{
		"resuelve":  false,
		"ip":        "",
		"zona":      strings.TrimSuffix(h.zone, "."),
		"dns":       "",
		"interfaz":  "",
		"ns_activa": nsActiva,
		"ns_vm":     h.nsName(),
		"mensaje":   "Pulsa «Configurar DNS».",
	}
	if h.network != nil {
		cfg := h.network.Network()
		response["dns"] = cfg.DNSIP
		response["interfaz"] = cfg.Interface
	}
	if err == nil && len(addrs) > 0 {
		response["resuelve"] = true
		response["ip"] = addrs[0]
		response["mensaje"] = ""
	}
	JSON(w, http.StatusOK, response)
}

// HandleRecords responde con los registros A de la zona DNS administrada (/api/dns/registros).
// Si la consulta al proveedor falla o no está disponible, retorna una lista vacía manteniendo el contrato.
// También admite PATCH para renombrar un registro A de la zona.
func (h *DnsHandler) HandleRecords(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPatch {
		h.renameRecord(w, r)
		return
	}
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
				// El registro del propio servidor DNS es del sistema: no se
				// lista ni se puede renombrar.
				if shortDNSName(item.Name, zone) == "ns" {
					continue
				}
				registros = append(registros, item)
			}
		}
	}

	JSON(w, http.StatusOK, map[string]any{
		"zona":      zone,
		"registros": registros,
	})
}

// renameRecordRequest describe el cuerpo de PATCH /api/dns/registros.
type renameRecordRequest struct {
	Nombre      string `json:"nombre"`
	NuevoNombre string `json:"nuevo_nombre"`
}

// renameRecord cambia el nombre de un registro A manteniendo su IP y TTL.
// No renombra la instancia ni el VHost de Apache.
func (h *DnsHandler) renameRecord(w http.ResponseWriter, r *http.Request) {
	if h.dnsProvider == nil {
		Error(w, http.StatusConflict, "El servicio DNS no está disponible")
		return
	}

	var req renameRecordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "Solicitud de renombrado inválida")
		return
	}
	oldName := strings.ToLower(strings.TrimSpace(req.Nombre))
	newName := strings.ToLower(strings.TrimSpace(req.NuevoNombre))
	if oldName == "" || newName == "" {
		Error(w, http.StatusBadRequest, "Debes indicar el nombre actual y el nuevo nombre")
		return
	}
	if err := domain.ValidateHostname(newName); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}

	records, err := h.dnsProvider.ListRecords(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, "No se pudieron consultar los registros DNS")
		return
	}

	oldShort := shortDNSName(oldName, h.zone)
	var old *domain.DnsRecord
	for i := range records {
		if shortDNSName(records[i].Name, h.zone) == oldShort {
			old = &records[i]
			break
		}
	}
	if old == nil {
		Error(w, http.StatusNotFound, "El registro DNS indicado no existe")
		return
	}
	if oldShort == "ns" {
		Error(w, http.StatusBadRequest, "El registro del servidor DNS es del sistema y no se puede renombrar")
		return
	}
	if oldShort == newName {
		JSON(w, http.StatusOK, map[string]any{
			"mensaje": "El registro ya tiene ese nombre",
			"datos":   old,
		})
		return
	}
	for i := range records {
		if shortDNSName(records[i].Name, h.zone) == newName {
			Error(w, http.StatusConflict, "Ya existe un registro DNS con el nuevo nombre")
			return
		}
	}

	ip, err := netip.ParseAddr(strings.TrimSpace(strings.Split(old.IP, "/")[0]))
	if err != nil {
		Error(w, http.StatusInternalServerError, "El registro DNS no tiene una IP válida")
		return
	}
	newRecord, err := domain.NewARecord(newName, h.zone, ip, old.TTL)
	if err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.dnsProvider.AddRecord(r.Context(), *newRecord); err != nil {
		Error(w, http.StatusInternalServerError, "No se pudo crear el nuevo registro DNS")
		return
	}
	if err := h.dnsProvider.RemoveRecord(r.Context(), oldShort); err != nil {
		_ = h.dnsProvider.RemoveRecord(r.Context(), newName)
		Error(w, http.StatusInternalServerError, "No se pudo eliminar el registro DNS anterior")
		return
	}

	if h.logger != nil {
		h.logger.Append("INFO", "dns", "Registro DNS renombrado", map[string]any{
			"anterior": oldShort,
			"nuevo":    newName,
			"ip":       newRecord.IP,
		})
	}
	JSON(w, http.StatusOK, map[string]any{
		"mensaje": "Registro DNS renombrado correctamente",
		"datos":   newRecord,
	})
}

// shortDNSName normaliza un nombre de registro a su forma corta dentro de la zona.
func shortDNSName(name, zone string) string {
	clean := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(name)), ".")
	cleanZone := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(zone)), ".")
	if cleanZone != "" && strings.HasSuffix(clean, "."+cleanZone) {
		clean = strings.TrimSuffix(clean, "."+cleanZone)
	}
	return clean
}
