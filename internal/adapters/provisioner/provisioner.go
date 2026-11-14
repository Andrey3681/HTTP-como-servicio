// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

// Package provisioner implementa la preparación real del laboratorio sobre VBoxManage.
package provisioner

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/domain"
	"github.com/Andrey3681/httpaas-orchestrator/internal/core/ports"
)

const vboxBinary = "VBoxManage"

var vboxVersionPattern = regexp.MustCompile(`^\d+(?:\.\d+){1,2}`)

// Provisioner ejecuta la configuración inicial del entorno mediante VBoxManage.
type Provisioner struct {
	executor ports.Executor
}

// NewProvisioner crea el aprovisionador con el ejecutor de procesos indicado.
func NewProvisioner(executor ports.Executor) *Provisioner {
	return &Provisioner{executor: executor}
}

func (p *Provisioner) vbox(ctx context.Context, args ...string) (string, error) {
	stdout, stderr, err := p.executor.Execute(ctx, vboxBinary, args...)
	if err != nil {
		detail := strings.TrimSpace(string(stderr))
		if detail == "" {
			detail = strings.TrimSpace(string(stdout))
		}
		return "", fmt.Errorf("VBoxManage %s: %w: %s", strings.Join(args, " "), err, detail)
	}
	return string(stdout), nil
}

// Inspect detecta el estado actual del host sin modificarlo.
func (p *Provisioner) Inspect(ctx context.Context, cfg domain.SetupConfig) (domain.EnvironmentStatus, error) {
	status := domain.EnvironmentStatus{}

	version, err := p.vbox(ctx, "--version")
	if err != nil {
		return status, nil
	}
	status.VBoxAvailable = true
	status.VBoxVersion = normalizeVersion(strings.TrimSpace(version))

	if out, err := p.vbox(ctx, "list", "vms"); err == nil {
		status.ExistingVMs = ParseVMNames(out)
	}
	for _, name := range status.ExistingVMs {
		switch name {
		case cfg.DNSVMName:
			status.HasDNSVM = true
		case cfg.BaseVMName:
			status.HasBaseVM = true
		}
	}

	if network, err := p.Network(ctx); err == nil {
		status.HostOnlyInterface = network.Interface
		status.HostOnlyIP = network.HostIP
	}

	if status.HasBaseVM {
		if disk, err := p.vmDiskPath(ctx, cfg.BaseVMName); err == nil && disk != "" {
			status.BaseDiskPath = disk
			if out, err := p.vbox(ctx, "showmediuminfo", "disk", disk); err == nil {
				status.BaseDiskMultiattach = IsMultiattach(out)
			}
		}
	}

	status.FreeDiskGB = freeDiskGB(cfg.RawDir)
	status.Ready = status.VBoxAvailable && status.HasDNSVM && status.HasBaseVM && status.BaseDiskMultiattach
	return status, nil
}

// EnsureNetwork garantiza la red host-only del proyecto y retorna su
// configuración real, derivada de la IP que VirtualBox asignó a la interfaz.
func (p *Provisioner) EnsureNetwork(ctx context.Context, _ domain.SetupConfig) (domain.NetworkConfig, error) {
	network, err := p.Network(ctx)
	if err == nil {
		if err := p.ensureDHCP(ctx, network); err != nil {
			return domain.NetworkConfig{}, err
		}
		return network, nil
	}

	created, createErr := p.vbox(ctx, "hostonlyif", "create")
	if createErr != nil {
		return domain.NetworkConfig{}, fmt.Errorf("no se pudo crear la interfaz host-only: %w", createErr)
	}
	if ParseCreatedInterface(created) == "" {
		return domain.NetworkConfig{}, fmt.Errorf("VBoxManage no reportó el nombre de la interfaz creada")
	}

	network, err = p.Network(ctx)
	if err != nil {
		return domain.NetworkConfig{}, err
	}
	if err := p.ensureDHCP(ctx, network); err != nil {
		return domain.NetworkConfig{}, err
	}
	return network, nil
}

// Network describe la interfaz host-only del proyecto sin modificarla.
func (p *Provisioner) Network(ctx context.Context) (domain.NetworkConfig, error) {
	out, err := p.vbox(ctx, "list", "hostonlyifs")
	if err != nil {
		return domain.NetworkConfig{}, err
	}
	for _, item := range ParseHostOnlyInterfaces(out) {
		// 192.168.56.1 es la red host-only por defecto de VirtualBox.
		if item.IP == "" || item.IP == "192.168.56.1" {
			continue
		}
		return domain.DeriveNetwork(item.Name, item.IP, item.Netmask)
	}
	return domain.NetworkConfig{}, fmt.Errorf("no hay una interfaz host-only del proyecto")
}

func (p *Provisioner) ensureDHCP(ctx context.Context, network domain.NetworkConfig) error {
	out, err := p.vbox(ctx, "list", "dhcpservers")
	if err != nil {
		return err
	}
	networkName := "HostInterfaceNetworking-" + network.Interface
	exists := strings.Contains(out, networkName)

	args := []string{
		"dhcpserver", "modify", "--interface=" + network.Interface,
		"--server-ip=" + network.HostIP,
		"--netmask=" + network.Netmask,
		"--lower-ip=" + network.DHCPLower,
		"--upper-ip=" + network.DHCPUpper,
		"--suppress-opt=3", // sin puerta de enlace en la red host-only
		"--suppress-opt=6", // sin DNS: los invitados usan la NIC NAT durante la preparación
		"--enable",
	}
	if !exists {
		args[1] = "add"
	}
	_, err = p.vbox(ctx, args...)
	return err
}

// ConvertToBaseVDI convierte la imagen raw en un VDI base reutilizable una sola
// vez. Si el VDI ya existe, no hace nada.
func (p *Provisioner) ConvertToBaseVDI(ctx context.Context, _ domain.SetupConfig, rawPath, baseVDI string, progress ports.ProgressFunc) error {
	if _, err := os.Stat(baseVDI); err == nil {
		return nil
	}
	if progress != nil {
		progress(-1, "Convirtiendo la imagen base a VDI (una sola vez)")
	}
	if _, err := p.vbox(ctx, "convertfromraw", rawPath, baseVDI, "--format", "VDI"); err != nil {
		return fmt.Errorf("fallo convirtiendo la imagen a VDI: %w", err)
	}
	return nil
}

// copyFile duplica un archivo completo.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// CreateVM registra la VM, duplica el VDI base y conecta el disco.
func (p *Provisioner) CreateVM(ctx context.Context, cfg domain.SetupConfig, vmName, netInterface, baseVDI string) (string, error) {
	if vmExists(ctx, p, vmName) {
		return p.vmDiskPath(ctx, vmName)
	}

	if _, err := p.vbox(ctx, "createvm", "--name", vmName, "--ostype", "Debian13_64", "--register"); err != nil {
		return "", err
	}

	if _, err := p.vbox(ctx, "modifyvm", vmName,
		"--memory", strconv.Itoa(cfg.MemoryMB),
		"--cpus", "1",
		"--nic1", "hostonly",
		"--hostonlyadapter1", netInterface,
		"--nic2", "nat",
		"--audio", "none",
		"--usb", "off",
		"--graphicscontroller", "vmsvga",
		"--vram", "16",
	); err != nil {
		return "", err
	}

	if _, err := p.vbox(ctx, "storagectl", vmName,
		"--name", "SATA",
		"--add", "sata",
		"--controller", "IntelAHCI",
		"--portcount", "2",
	); err != nil {
		return "", err
	}

	folder, err := p.vmFolder(ctx, vmName)
	if err != nil {
		return "", err
	}
	diskPath := filepath.Join(folder, vmName+".vdi")
	if err := copyFile(baseVDI, diskPath); err != nil {
		return "", fmt.Errorf("fallo duplicando el VDI base: %w", err)
	}
	if _, err := p.vbox(ctx, "modifymedium", "disk", diskPath, "--resize", strconv.Itoa(cfg.DiskMB)); err != nil {
		return "", fmt.Errorf("fallo redimensionando el disco: %w", err)
	}

	if _, err := p.vbox(ctx, "storageattach", vmName,
		"--storagectl", "SATA",
		"--port", "0",
		"--device", "0",
		"--type", "hdd",
		"--medium", diskPath,
	); err != nil {
		return "", err
	}
	return diskPath, nil
}

// CreateSeedISO genera la ISO NoCloud con user-data y meta-data.
func (p *Provisioner) CreateSeedISO(ctx context.Context, cfg domain.SetupConfig, vmName, userData, metaData string) (string, error) {
	if err := os.MkdirAll(cfg.SeedDir, 0o755); err != nil {
		return "", err
	}
	userDataPath := filepath.Join(cfg.SeedDir, vmName+"-user-data")
	metaDataPath := filepath.Join(cfg.SeedDir, vmName+"-meta-data")
	if err := os.WriteFile(userDataPath, []byte(userData), 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile(metaDataPath, []byte(metaData), 0o644); err != nil {
		return "", err
	}

	isoPath := filepath.Join(cfg.SeedDir, vmName+"-seed.iso")
	if err := buildSeedISO(ctx, p.executor, cfg.SeedDir, isoPath, []string{userDataPath, metaDataPath}); err != nil {
		return "", fmt.Errorf("fallo generando la ISO semilla: %w", err)
	}
	return isoPath, nil
}

// StartSeedBoot conecta la ISO semilla y arranca la máquina en headless.
func (p *Provisioner) StartSeedBoot(ctx context.Context, _ domain.SetupConfig, vmName, seedISO string) error {
	out, err := p.vbox(ctx, "showvminfo", vmName, "--machinereadable")
	if err == nil && strings.Contains(out, `VMState="running"`) {
		return fmt.Errorf("la máquina %s sigue en ejecución con una preparación anterior; usa Reiniciar para recrearla", vmName)
	}
	if _, err := p.vbox(ctx, "storageattach", vmName,
		"--storagectl", "SATA",
		"--port", "1",
		"--device", "0",
		"--type", "dvddrive",
		"--medium", seedISO,
	); err != nil {
		return fmt.Errorf("fallo conectando la semilla: %w", err)
	}
	if _, err := p.vbox(ctx, "startvm", vmName, "--type", "headless"); err != nil {
		return fmt.Errorf("fallo arrancando la VM: %w", err)
	}
	return nil
}

// WaitGuestReady espera a que Guest Additions y la red del invitado respondan,
// reportando hitos reales del invitado mientras tanto.
func (p *Provisioner) WaitGuestReady(ctx context.Context, vmName string, timeout time.Duration, progress ...ports.ProgressFunc) error {
	report := func(percent int, message string) {
		for _, fn := range progress {
			if fn != nil {
				fn(percent, message)
			}
		}
	}
	if timeout <= 0 {
		timeout = 45 * time.Minute
	}
	started := time.Now()
	deadline := started.Add(timeout)
	stable := 0
	additionsReady := false
	lastTick := time.Time{}

	report(5, "Arrancando la máquina")

	for time.Now().Before(deadline) {
		gaOut, gaErr := p.vbox(ctx, "guestproperty", "get", vmName, "/VirtualBox/GuestAdd/Version")
		ipOut, ipErr := p.vbox(ctx, "guestproperty", "get", vmName, "/VirtualBox/GuestInfo/Net/0/V4/IP")
		if gaErr == nil && strings.Contains(gaOut, "Value:") && !additionsReady {
			additionsReady = true
			report(75, "Guest Additions instaladas; esperando la red")
		}
		if gaErr == nil && ipErr == nil && strings.Contains(gaOut, "Value:") && strings.Contains(ipOut, "Value:") {
			stable++
			if stable >= 2 {
				report(100, "Máquina lista")
				return nil
			}
		} else {
			stable = 0
		}

		// Progreso estimado mientras el instalador trabaja (sin hitos intermedios).
		if !additionsReady && time.Since(lastTick) >= 10*time.Second {
			elapsed := time.Since(started)
			percent := 5 + int(elapsed.Minutes())*3
			if percent > 65 {
				percent = 65
			}
			report(percent, fmt.Sprintf("Instalando paquetes y Guest Additions · %dm %02ds", int(elapsed.Minutes()), int(elapsed.Seconds())%60))
			lastTick = time.Now()
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
	return fmt.Errorf("el invitado %s no quedó disponible en %s", vmName, timeout)
}

// RunGuestScript copia un script local al invitado y lo ejecuta como root con
// los argumentos indicados.
func (p *Provisioner) RunGuestScript(ctx context.Context, cfg domain.SetupConfig, vmName, localScript string, args ...string) error {
	if _, err := os.Stat(localScript); err != nil {
		return fmt.Errorf("script de invitado no encontrado: %s: %w", localScript, err)
	}
	remote := "/tmp/" + filepath.Base(localScript)

	if _, err := p.vbox(ctx, "guestcontrol", vmName, "copyto",
		"--username", cfg.GuestUser,
		"--password", cfg.GuestPassword,
		"--target-directory", "/tmp",
		localScript,
	); err != nil {
		return fmt.Errorf("no se pudo copiar %s al invitado: %w", localScript, err)
	}

	runArgs := []string{
		"guestcontrol", vmName, "run",
		"--exe", "/usr/bin/sudo",
		"--username", cfg.GuestUser,
		"--password", cfg.GuestPassword,
		"--wait-stdout",
		"--wait-stderr",
		"--timeout", "1800000",
		"--", "-n", "/bin/bash", remote,
	}
	runArgs = append(runArgs, args...)
	out, err := p.vbox(ctx, runArgs...)
	if err != nil {
		return fmt.Errorf("fallo ejecutando %s en %s: %w (%s)", remote, vmName, err, lastLines(out, 6))
	}
	return nil
}

// ReserveDHCPAddress fija la IP indicada a la MAC de la VM.
func (p *Provisioner) ReserveDHCPAddress(ctx context.Context, _ domain.SetupConfig, vmName, ip string) error {
	network, err := p.Network(ctx)
	if err != nil {
		return err
	}
	_, err = p.vbox(ctx, "dhcpserver", "modify",
		"--interface="+network.Interface,
		"--vm="+vmName,
		"--nic=1",
		"--fixed-address="+ip,
	)
	return err
}

// MarkBaseMultiattach apaga la plantilla, marca el disco como multiattach y lo reconecta.
func (p *Provisioner) MarkBaseMultiattach(ctx context.Context, cfg domain.SetupConfig, vmName string) (string, error) {
	disk, err := p.vmDiskPath(ctx, vmName)
	if err != nil {
		return "", err
	}
	if disk == "" {
		return "", fmt.Errorf("la VM %s no tiene disco conectado", vmName)
	}

	_ = p.stopVM(ctx, vmName)

	if _, err := p.vbox(ctx, "storageattach", vmName,
		"--storagectl", "SATA", "--port", "0", "--device", "0",
		"--type", "hdd", "--medium", "none",
	); err != nil {
		return "", err
	}
	if _, err := p.vbox(ctx, "modifymedium", "disk", disk, "--type", "multiattach"); err != nil {
		return "", err
	}
	if _, err := p.vbox(ctx, "storageattach", vmName,
		"--storagectl", "SATA", "--port", "0", "--device", "0",
		"--type", "hdd", "--medium", disk,
	); err != nil {
		return "", err
	}

	out, err := p.vbox(ctx, "showmediuminfo", "disk", disk)
	if err != nil {
		return "", err
	}
	if !IsMultiattach(out) {
		return "", fmt.Errorf("el disco %s no quedó en modo multiattach", disk)
	}
	return disk, nil
}

// ResetEnvironment elimina las VMs base y sus semillas para reiniciar la
// preparación. Conserva la imagen descargada y la red.
func (p *Provisioner) ResetEnvironment(ctx context.Context, cfg domain.SetupConfig) error {
	for _, vmName := range []string{cfg.DNSVMName, cfg.BaseVMName} {
		if vmName == "" {
			continue
		}
		if vmExists(ctx, p, vmName) {
			_ = p.stopVM(ctx, vmName)
			if _, _, err := p.executor.Execute(ctx, vboxBinary, "unregistervm", vmName, "--delete"); err != nil {
				return fmt.Errorf("no se pudo eliminar la VM %s: %w", vmName, err)
			}
		}
		for _, suffix := range []string{"-seed.iso", "-user-data", "-meta-data"} {
			_ = os.Remove(filepath.Join(cfg.SeedDir, vmName+suffix))
		}
	}
	return nil
}

func (p *Provisioner) stopVM(ctx context.Context, vmName string) error {
	if _, err := p.vbox(ctx, "controlvm", vmName, "acpipoweroff"); err != nil {
		_, _ = p.vbox(ctx, "controlvm", vmName, "poweroff")
	}
	for i := 0; i < 30; i++ {
		out, err := p.vbox(ctx, "showvminfo", vmName, "--machinereadable")
		if err != nil {
			return err
		}
		if strings.Contains(out, `VMState="poweroff"`) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return nil
}

func (p *Provisioner) vmFolder(ctx context.Context, vmName string) (string, error) {
	out, err := p.vbox(ctx, "showvminfo", vmName, "--machinereadable")
	if err != nil {
		return "", err
	}
	cfgFile := ParseMachineReadableValue(out, "CfgFile")
	if cfgFile == "" {
		return "", fmt.Errorf("no se pudo obtener la carpeta de la VM %s", vmName)
	}
	return filepath.Dir(cfgFile), nil
}

func (p *Provisioner) vmDiskPath(ctx context.Context, vmName string) (string, error) {
	out, err := p.vbox(ctx, "showvminfo", vmName, "--machinereadable")
	if err != nil {
		return "", err
	}
	for _, key := range []string{"SATA-0-0", "SATA-ImageUUID-0-0", "IDE-0-0"} {
		value := ParseMachineReadableValue(out, key)
		if value != "" && value != "none" {
			if strings.HasSuffix(value, ".vdi") || strings.Contains(value, "/") {
				return value, nil
			}
		}
	}
	return "", nil
}

func vmExists(ctx context.Context, p *Provisioner, vmName string) bool {
	out, err := p.vbox(ctx, "list", "vms")
	if err != nil {
		return false
	}
	for _, name := range ParseVMNames(out) {
		if name == vmName {
			return true
		}
	}
	return false
}

// ParseVMNames extrae los nombres de la salida de VBoxManage list vms.
func ParseVMNames(output string) []string {
	var names []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, `"`) {
			continue
		}
		end := strings.Index(line[1:], `"`)
		if end < 0 {
			continue
		}
		names = append(names, line[1:1+end])
	}
	return names
}

// HostOnlyInterface describe una interfaz host-only reportada por VirtualBox.
type HostOnlyInterface struct {
	Name    string
	IP      string
	Netmask string
}

// ParseHostOnlyInterfaces extrae nombre e IPv4 de la salida de list hostonlyifs.
func ParseHostOnlyInterfaces(output string) []HostOnlyInterface {
	var result []HostOnlyInterface
	var current HostOnlyInterface
	flush := func() {
		if current.Name != "" {
			result = append(result, current)
		}
		current = HostOnlyInterface{}
	}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Name:"):
			flush()
			current.Name = strings.TrimSpace(strings.TrimPrefix(line, "Name:"))
		case strings.HasPrefix(line, "IPAddress:"):
			current.IP = strings.TrimSpace(strings.TrimPrefix(line, "IPAddress:"))
		case strings.HasPrefix(line, "NetworkMask:"):
			current.Netmask = strings.TrimSpace(strings.TrimPrefix(line, "NetworkMask:"))
		}
	}
	flush()
	return result
}

// ParseCreatedInterface extrae el nombre de la interfaz recién creada.
func ParseCreatedInterface(output string) string {
	const marker = "Interface '"
	idx := strings.Index(output, marker)
	if idx < 0 {
		return ""
	}
	rest := output[idx+len(marker):]
	end := strings.Index(rest, "'")
	if end < 0 {
		return ""
	}
	return rest[:end]
}

// ParseMachineReadableValue retorna el valor de una clave en salida machinereadable.
func ParseMachineReadableValue(output, key string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, key+"=") {
			continue
		}
		value := strings.TrimPrefix(line, key+"=")
		return strings.Trim(value, `"`)
	}
	return ""
}

// IsMultiattach indica si showmediuminfo reporta un disco multiconexión.
func IsMultiattach(output string) bool {
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "Type:") {
			return strings.Contains(line, "multiattach")
		}
	}
	return false
}

// normalizeVersion extrae la versión numérica de la salida de VBoxManage,
// descartando sufijos de distribución como "_RPMFUSIONr175245".
func normalizeVersion(raw string) string {
	if match := vboxVersionPattern.FindString(raw); match != "" {
		return match
	}
	return raw
}

func freeDiskGB(path string) int64 {
	for dir := path; dir != "" && dir != "/"; dir = filepath.Dir(dir) {
		var stat syscall.Statfs_t
		if err := syscall.Statfs(dir, &stat); err == nil {
			return int64(stat.Bavail) * int64(stat.Bsize) / (1024 * 1024 * 1024)
		}
	}
	return 0
}

// (sin privilegios: la red host-only se usa tal como VirtualBox la crea)

func lastLines(text string, n int) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) <= n {
		return strings.Join(lines, " | ")
	}
	return strings.Join(lines[len(lines)-n:], " | ")
}
