// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package vbox

import (
	"context"
	"fmt"
	"log"
	"net/netip"
	"path/filepath"
	"sync"
	"time"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/domain"
	"github.com/Andrey3681/httpaas-orchestrator/internal/core/ports"
)

// HypervisorAdapter implementa ports.Hypervisor sobre VBoxManage con serialización estricta.
type HypervisorAdapter struct {
	executor   ports.Executor
	vboxBinary string
	domainZone string
	hostOnlyIP string
	logger     ports.EventLogger
	mu         sync.Mutex // Serializa el acceso a VBoxSVC para evitar colisiones concurrentes
}

// NewHypervisorAdapter crea un nuevo adaptador de VirtualBox.
func NewHypervisorAdapter(executor ports.Executor, domainZone, hostOnlyIP string, logger ports.EventLogger) *HypervisorAdapter {
	return &HypervisorAdapter{
		executor:   executor,
		vboxBinary: "VBoxManage",
		domainZone: domainZone,
		hostOnlyIP: hostOnlyIP,
		logger:     logger,
	}
}

func (h *HypervisorAdapter) log(level, message string, fields map[string]any) {
	if h.logger != nil {
		h.logger.Append(level, "vbox", message, fields)
	}
}

// CreateInstanceFromBase crea una máquina virtual enlazando el disco multiconexión base.
func (h *HypervisorAdapter) CreateInstanceFromBase(ctx context.Context, name string, baseDiskPath string) (*domain.Instance, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	log.Printf("[VBox] Registrando máquina virtual '%s'", name)
	h.log("INFO", "Creando máquina virtual", map[string]any{"instancia": name, "base_disk": baseDiskPath})
	// 1. VBoxManage createvm --name <name> --ostype Debian_64 --register
	_, _, err := h.executor.Execute(ctx, h.vboxBinary, "createvm", "--name", name, "--ostype", "Debian_64", "--register")
	if err != nil {
		return nil, fmt.Errorf("fallo createvm: %w", err)
	}

	// 2. VBoxManage modifyvm <name> --memory 1024 --cpus 1 --nic1 hostonly --hostonlyadapter1 <if>
	iface, err := h.resolveHostOnlyInterface(ctx)
	if err != nil {
		_ = h.destroyLocked(ctx, name)
		return nil, err
	}
	_, _, err = h.executor.Execute(ctx, h.vboxBinary, "modifyvm", name,
		"--memory", "512",
		"--cpus", "1",
		"--nic1", "hostonly",
		"--hostonlyadapter1", iface,
	)
	if err != nil {
		_ = h.destroyLocked(ctx, name)
		return nil, fmt.Errorf("fallo modifyvm: %w", err)
	}

	// 3. VBoxManage storagectl <name> --name SATA --add sata --controller IntelAHCI
	_, _, err = h.executor.Execute(ctx, h.vboxBinary, "storagectl", name,
		"--name", "SATA",
		"--add", "sata",
		"--controller", "IntelAHCI",
		"--portcount", "2",
	)
	if err != nil {
		_ = h.destroyLocked(ctx, name)
		return nil, fmt.Errorf("fallo storagectl: %w", err)
	}

	// 4. Crear un disco diferencial explícito cuyo padre es el disco multiconexión.
	folder, err := h.vmFolder(ctx, name)
	if err != nil {
		_ = h.destroyLocked(ctx, name)
		return nil, err
	}
	diffDisk := filepath.Join(folder, name+".vdi")
	_, _, err = h.executor.Execute(ctx, h.vboxBinary, "createmedium", "disk",
		"--filename", diffDisk,
		"--diffparent", baseDiskPath,
		"--format", "VDI",
	)
	if err != nil {
		_ = h.destroyLocked(ctx, name)
		return nil, fmt.Errorf("fallo createmedium diferencial: %w", err)
	}

	// 5. Conectar únicamente el diferencial; el disco base nunca se monta en escritura.
	_, _, err = h.executor.Execute(ctx, h.vboxBinary, "storageattach", name,
		"--storagectl", "SATA",
		"--port", "0",
		"--device", "0",
		"--type", "hdd",
		"--medium", diffDisk,
	)
	if err != nil {
		_ = h.destroyLocked(ctx, name)
		return nil, fmt.Errorf("fallo storageattach con disco diferencial: %w", err)
	}

	return domain.NewInstance(name, h.domainZone)
}

// resolveHostOnlyInterface ubica la interfaz host-only del proyecto por su IP.
func (h *HypervisorAdapter) resolveHostOnlyInterface(ctx context.Context) (string, error) {
	out, _, err := h.executor.Execute(ctx, h.vboxBinary, "list", "hostonlyifs")
	if err != nil {
		return "", fmt.Errorf("fallo list hostonlyifs: %w", err)
	}
	if iface := ParseHostOnlyInterfaceByIP(string(out), h.hostOnlyIP); iface != "" {
		return iface, nil
	}
	if iface := ParseFirstProjectInterface(string(out)); iface != "" {
		return iface, nil
	}
	return "", fmt.Errorf("no existe una interfaz host-only del proyecto; ejecuta la preparación inicial")
}

// vmFolder obtiene la carpeta de configuración de la máquina virtual.
func (h *HypervisorAdapter) vmFolder(ctx context.Context, vmName string) (string, error) {
	out, _, err := h.executor.Execute(ctx, h.vboxBinary, "showvminfo", vmName, "--machinereadable")
	if err != nil {
		return "", fmt.Errorf("fallo showvminfo %s: %w", vmName, err)
	}
	cfgFile := ParseMachineValue(string(out), "CfgFile")
	if cfgFile == "" {
		return "", fmt.Errorf("no se pudo obtener la carpeta de la VM %s", vmName)
	}
	return filepath.Dir(cfgFile), nil
}

// Start inicia la máquina virtual en modo headless.
func (h *HypervisorAdapter) Start(ctx context.Context, vmName string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	log.Printf("[VBox] Iniciando máquina virtual '%s' en modo headless", vmName)
	_, _, err := h.executor.Execute(ctx, h.vboxBinary, "startvm", vmName, "--type", "headless")
	if err != nil {
		return fmt.Errorf("fallo al arrancar VM '%s': %w", vmName, err)
	}
	return nil
}

// GetIPAddress sondea la dirección IP asignada a la VM mediante Guest Additions.
func (h *HypervisorAdapter) GetIPAddress(ctx context.Context, vmName string) (netip.Addr, error) {
	// Reintentos durante 60 segundos
	timeout := time.After(60 * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	log.Printf("[VBox] Esperando asignación de IP para VM '%s'...", vmName)

	for {
		select {
		case <-ctx.Done():
			return netip.Addr{}, ctx.Err()
		case <-timeout:
			return netip.Addr{}, fmt.Errorf("tiempo de espera agotado buscando IP para VM '%s'", vmName)
		case <-ticker.C:
			stdout, _, err := func() ([]byte, []byte, error) {
				h.mu.Lock()
				defer h.mu.Unlock()
				return h.executor.Execute(ctx, h.vboxBinary, "guestproperty", "get", vmName, "/VirtualBox/GuestInfo/Net/0/V4/IP")
			}()

			if err == nil {
				ip, parseErr := ParseGuestIP(string(stdout))
				if parseErr == nil && ip.IsValid() && !ip.IsLoopback() {
					log.Printf("[VBox] IP detectada para '%s': %s", vmName, ip.String())
					h.log("INFO", "IP detectada en máquina virtual", map[string]any{"instancia": vmName, "ip": ip.String()})
					return ip, nil
				}
			}
		}
	}
}

// Stop apaga la máquina virtual.
func (h *HypervisorAdapter) Stop(ctx context.Context, vmName string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	log.Printf("[VBox] Apagando VM '%s'", vmName)
	_, _, _ = h.executor.Execute(ctx, h.vboxBinary, "controlvm", vmName, "poweroff")
	return nil
}

// Destroy destruye la VM y elimina todos sus archivos.
func (h *HypervisorAdapter) Destroy(ctx context.Context, vmName string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.destroyLocked(ctx, vmName)
}

func (h *HypervisorAdapter) destroyLocked(ctx context.Context, vmName string) error {
	log.Printf("[VBox] Desregistrando y eliminando VM '%s'", vmName)
	_, _, _ = h.executor.Execute(ctx, h.vboxBinary, "controlvm", vmName, "poweroff")
	_, _, err := h.executor.Execute(ctx, h.vboxBinary, "unregistervm", vmName, "--delete")
	return err
}
