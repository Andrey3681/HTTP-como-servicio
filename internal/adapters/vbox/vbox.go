// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package vbox

import (
	"context"
	"fmt"
	"log"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Andrey3681/httpaas-orchestrator/internal/adapters/seediso"
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

	guestUser  func() string
	sshKeyPath string
	seedDir    string
}

// ConfigureInstanceSeed habilita la semilla cloud-init de las instancias con el
// usuario del laboratorio, su clave pública y el directorio de semillas.
func (h *HypervisorAdapter) ConfigureInstanceSeed(guestUser func() string, sshKeyPath, seedDir string) {
	h.guestUser = guestUser
	h.sshKeyPath = sshKeyPath
	h.seedDir = seedDir
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
		"--boot1", "disk",
		"--boot2", "dvd",
		"--boot3", "none",
		"--boot4", "none",
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

	// 6. Semilla cloud-init propia: identidad y red DHCP de la instancia.
	if err := h.attachInstanceSeed(ctx, name); err != nil {
		_ = h.destroyLocked(ctx, name)
		return nil, err
	}
	h.log("INFO", "Disco diferencial y semilla conectados", map[string]any{"instancia": name})

	return domain.NewInstance(name, h.domainZone)
}

// instanceUserData es la semilla de una instancia: sobrescribe el netplan
// estático heredado de la plantilla y solicita DHCP en el adaptador host-only.
const instanceUserData = `#cloud-config
hostname: %s

network:
  config: disabled

bootcmd:
  - |
    cat > /etc/netplan/99-httpaas.yaml <<'NETPLAN'
    network:
      version: 2
      renderer: networkd
      ethernets:
        enp0s3:
          dhcp4: true
          optional: true
        enp0s8:
          dhcp4: true
          dhcp6: false
          accept-ra: false
          optional: true
    NETPLAN
    chmod 600 /etc/netplan/99-httpaas.yaml
    netplan apply

users:
  - name: %s
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
    lock_passwd: false
    ssh_authorized_keys:
      - "%s"

runcmd:
  - systemctl restart apache2
`

// attachInstanceSeed genera y conecta la ISO NoCloud de la instancia.
func (h *HypervisorAdapter) attachInstanceSeed(ctx context.Context, name string) error {
	if h.seedDir == "" || h.sshKeyPath == "" {
		return nil
	}
	publicKey, err := os.ReadFile(h.sshKeyPath + ".pub")
	if err != nil {
		return fmt.Errorf("no se pudo leer la clave pública del laboratorio: %w", err)
	}
	user := "debian"
	if h.guestUser != nil {
		if value := h.guestUser(); value != "" {
			user = value
		}
	}

	userData := fmt.Sprintf(instanceUserData, name, user, strings.TrimSpace(string(publicKey)))
	metaData := fmt.Sprintf("instance-id: iid-%s-%d\nlocal-hostname: %s\n", name, time.Now().Unix(), name)

	staging := filepath.Join(h.seedDir, "instances", name)
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return fmt.Errorf("no se pudo preparar la semilla de %s: %w", name, err)
	}
	userDataPath := filepath.Join(staging, "user-data")
	metaDataPath := filepath.Join(staging, "meta-data")
	if err := os.WriteFile(userDataPath, []byte(userData), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(metaDataPath, []byte(metaData), 0o644); err != nil {
		return err
	}

	isoPath := filepath.Join(h.seedDir, "instances", name+"-seed.iso")
	if err := seediso.Build(ctx, h.executor, staging, isoPath, []string{userDataPath, metaDataPath}); err != nil {
		return fmt.Errorf("fallo generando la semilla de %s: %w", name, err)
	}
	if _, _, err := h.executor.Execute(ctx, h.vboxBinary, "storageattach", name,
		"--storagectl", "SATA",
		"--port", "1",
		"--device", "0",
		"--type", "dvddrive",
		"--medium", isoPath,
	); err != nil {
		return fmt.Errorf("fallo conectando la semilla de %s: %w", name, err)
	}
	return nil
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
	h.log("INFO", "Arrancando la máquina virtual en modo headless", map[string]any{"instancia": vmName})
	_, _, err := h.executor.Execute(ctx, h.vboxBinary, "startvm", vmName, "--type", "headless")
	if err != nil {
		return fmt.Errorf("fallo al arrancar VM '%s': %w", vmName, err)
	}
	return nil
}

// GetIPAddress obtiene la dirección IP de la VM. La imagen cloud no incluye el
// servicio de usuario de Guest Additions, así que la fuente principal es la
// concesión DHCP consultada por MAC; guestproperty queda como último recurso.
func (h *HypervisorAdapter) GetIPAddress(ctx context.Context, vmName string) (netip.Addr, error) {
	iface, ifaceErr := h.resolveHostOnlyInterface(ctx)
	if ifaceErr != nil {
		h.log("WARN", "No se pudo resolver la interfaz host-only: "+ifaceErr.Error(), map[string]any{"instancia": vmName})
	}
	mac, macErr := h.vmMacAddress(ctx, vmName)
	if macErr != nil {
		h.log("WARN", "No se pudo leer la MAC de la instancia: "+macErr.Error(), map[string]any{"instancia": vmName})
	}

	log.Printf("[VBox] Esperando asignación de IP para VM '%s'...", vmName)
	deadline := time.Now().Add(120 * time.Second)
	var dhcpErr, gaErr error
	reportedDHCP := false
	attempt := 0

	for {
		if mac != "" && ifaceErr == nil {
			if ip, err := h.dhcpLeaseIP(ctx, iface, mac); err == nil {
				log.Printf("[VBox] IP obtenida por DHCP para '%s': %s", vmName, ip.String())
				h.log("INFO", "IP obtenida por concesión DHCP", map[string]any{"instancia": vmName, "ip": ip.String()})
				return ip, nil
			} else if !reportedDHCP {
				dhcpErr = err
				h.log("WARN", "Aún sin concesión DHCP para la instancia: "+err.Error(), map[string]any{"instancia": vmName, "mac": mac, "interfaz": iface})
				reportedDHCP = true
			}

			// La concesión puede no estar visible tras recrear el DHCP; la
			// tabla de vecinos del anfitrión sí conoce la MAC.
			if attempt%3 == 0 {
				if ip, err := h.arpScanIP(ctx, iface, mac); err == nil {
					h.log("INFO", "IP obtenida por ARP", map[string]any{"instancia": vmName, "ip": ip.String()})
					return ip, nil
				}
			} else if ip, err := h.neighborIP(ctx, iface, mac); err == nil {
				h.log("INFO", "IP obtenida por tabla de vecinos", map[string]any{"instancia": vmName, "ip": ip.String()})
				return ip, nil
			}
		}

		if ip, err := h.guestPropertyIP(ctx, vmName); err == nil {
			log.Printf("[VBox] IP detectada por guestproperty para '%s': %s", vmName, ip.String())
			h.log("INFO", "IP detectada en máquina virtual", map[string]any{"instancia": vmName, "ip": ip.String()})
			return ip, nil
		} else {
			gaErr = err
		}

		if time.Now().After(deadline) {
			return netip.Addr{}, fmt.Errorf("tiempo de espera agotado buscando IP para VM '%s' (DHCP: %v; Guest Additions: %v)", vmName, dhcpErr, gaErr)
		}
		attempt++
		select {
		case <-ctx.Done():
			return netip.Addr{}, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

// runLocked ejecuta VBoxManage manteniendo la exclusión mutua del adaptador.
func (h *HypervisorAdapter) runLocked(ctx context.Context, args ...string) ([]byte, []byte, error) {
	return h.runCommandLocked(ctx, h.vboxBinary, args...)
}

// runCommandLocked ejecuta un comando del host con la exclusión mutua del adaptador.
func (h *HypervisorAdapter) runCommandLocked(ctx context.Context, command string, args ...string) ([]byte, []byte, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.executor.Execute(ctx, command, args...)
}

// neighborIP busca en la tabla de vecinos del anfitrión la IP de la MAC.
func (h *HypervisorAdapter) neighborIP(ctx context.Context, iface, mac string) (netip.Addr, error) {
	if runtime.GOOS == "windows" {
		stdout, _, err := h.runCommandLocked(ctx, "arp", "-a")
		if err != nil {
			return netip.Addr{}, err
		}
		return ParseWindowsARP(string(stdout), mac)
	}
	stdout, _, err := h.runCommandLocked(ctx, "ip", "neigh", "show", "dev", iface)
	if err != nil {
		return netip.Addr{}, err
	}
	return ParseNeighborIP(string(stdout), mac)
}

// arpScanIP sondea el rango dinámico para poblar la tabla de vecinos y busca la MAC.
func (h *HypervisorAdapter) arpScanIP(ctx context.Context, iface, mac string) (netip.Addr, error) {
	base := h.hostOnlyIP
	if i := strings.LastIndex(base, "."); i > 0 {
		base = base[:i]
	}
	if base == "" {
		return netip.Addr{}, fmt.Errorf("no hay subred para el barrido ARP")
	}
	var wg sync.WaitGroup
	limit := make(chan struct{}, 32)
	for i := 100; i <= 199; i++ {
		target := fmt.Sprintf("%s.%d", base, i)
		wg.Add(1)
		limit <- struct{}{}
		go func(ip string) {
			defer wg.Done()
			defer func() { <-limit }()
			var args []string
			if runtime.GOOS == "windows" {
				args = []string{"-n", "1", "-w", "750", ip}
			} else {
				args = []string{"-c", "1", "-W", "1", ip}
			}
			// El sondeo no toca VBoxSVC: se ejecuta sin la exclusión mutua.
			_, _, _ = h.executor.Execute(ctx, "ping", args...)
		}(target)
	}
	wg.Wait()
	return h.neighborIP(ctx, iface, mac)
}

// vmMacAddress lee la MAC del primer adaptador de la VM.
func (h *HypervisorAdapter) vmMacAddress(ctx context.Context, vmName string) (string, error) {
	stdout, _, err := h.runLocked(ctx, "showvminfo", vmName, "--machinereadable")
	if err != nil {
		return "", fmt.Errorf("fallo showvminfo %s: %w", vmName, err)
	}
	mac := ParseVMMac(string(stdout))
	if mac == "" {
		return "", fmt.Errorf("la VM %s no reporta MAC", vmName)
	}
	return mac, nil
}

// dhcpLeaseIP consulta la concesión DHCP asignada a la MAC indicada.
func (h *HypervisorAdapter) dhcpLeaseIP(ctx context.Context, iface, mac string) (netip.Addr, error) {
	stdout, _, err := h.runLocked(ctx, "dhcpserver", "findlease", "--interface="+iface, "--mac-address="+mac)
	if err != nil {
		return netip.Addr{}, err
	}
	return ParseDhcpLeaseIP(string(stdout))
}

// guestPropertyIP consulta la IP publicada por Guest Additions.
func (h *HypervisorAdapter) guestPropertyIP(ctx context.Context, vmName string) (netip.Addr, error) {
	stdout, _, err := h.runLocked(ctx, "guestproperty", "get", vmName, "/VirtualBox/GuestInfo/Net/0/V4/IP")
	if err != nil {
		return netip.Addr{}, err
	}
	return ParseGuestIP(string(stdout))
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
	if h.seedDir != "" {
		_ = os.Remove(filepath.Join(h.seedDir, "instances", vmName+"-seed.iso"))
		_ = os.RemoveAll(filepath.Join(h.seedDir, "instances", vmName))
	}
	return err
}
