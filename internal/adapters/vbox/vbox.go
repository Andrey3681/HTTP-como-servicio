// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package vbox

import (
	"context"
	"fmt"
	"log"
	"net/netip"
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
	mu         sync.Mutex // Serializa el acceso a VBoxSVC para evitar colisiones concurrentes
}

// NewHypervisorAdapter crea un nuevo adaptador de VirtualBox.
func NewHypervisorAdapter(executor ports.Executor, domainZone string) *HypervisorAdapter {
	return &HypervisorAdapter{
		executor:   executor,
		vboxBinary: "VBoxManage",
		domainZone: domainZone,
	}
}

// CreateInstanceFromBase crea una máquina virtual enlazando el disco multiconexión base.
func (h *HypervisorAdapter) CreateInstanceFromBase(ctx context.Context, name string, baseDiskPath string) (*domain.Instance, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	log.Printf("[VBox] Registrando máquina virtual '%s'", name)
	// 1. VBoxManage createvm --name <name> --ostype Debian_64 --register
	_, _, err := h.executor.Execute(ctx, h.vboxBinary, "createvm", "--name", name, "--ostype", "Debian_64", "--register")
	if err != nil {
		return nil, fmt.Errorf("fallo createvm: %w", err)
	}

	// 2. VBoxManage modifyvm <name> --memory 1024 --cpus 1 --nic1 intnet --intnet1 cloudnet
	_, _, err = h.executor.Execute(ctx, h.vboxBinary, "modifyvm", name,
		"--memory", "1024",
		"--cpus", "1",
		"--nic1", "intnet",
		"--intnet1", "cloudnet",
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
	)
	if err != nil {
		_ = h.destroyLocked(ctx, name)
		return nil, fmt.Errorf("fallo storagectl: %w", err)
	}

	// 4. VBoxManage storageattach <name> --storagectl SATA --port 0 --device 0 --type hdd --medium <baseDiskPath>
	_, _, err = h.executor.Execute(ctx, h.vboxBinary, "storageattach", name,
		"--storagectl", "SATA",
		"--port", "0",
		"--device", "0",
		"--type", "hdd",
		"--medium", baseDiskPath,
	)
	if err != nil {
		_ = h.destroyLocked(ctx, name)
		return nil, fmt.Errorf("fallo storageattach con disco multiconexión: %w", err)
	}

	return domain.NewInstance(name, h.domainZone)
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
