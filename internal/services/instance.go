// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package services

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/netip"
	"strings"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/domain"
	"github.com/Andrey3681/httpaas-orchestrator/internal/core/ports"
)

// InstanceOptions agrupa los colaboradores opcionales de las operaciones sobre
// instancias ya aprovisionadas.
type InstanceOptions struct {
	// Operator ejecuta operaciones dentro de la instancia (reinicio de Apache).
	Operator ports.InstanceOperator
	// Deployer vuelve a publicar el contenido web.
	Deployer ports.Deployer
	// GuestUser retorna el usuario configurado para los invitados.
	GuestUser func() string
	// SSHKeyPath es la ruta local de la clave privada del entorno.
	SSHKeyPath string
}

// InstanceService gestiona las operaciones de consulta, acceso y eliminación
// controlada de instancias.
type InstanceService struct {
	repo        ports.Repository
	hypervisor  ports.Hypervisor
	dnsProvider ports.DnsProvider
	logger      ports.EventLogger
	operator    ports.InstanceOperator
	deployer    ports.Deployer
	guestUser   func() string
	sshKeyPath  string
}

// NewInstanceService crea una nueva instancia del servicio de gestión.
func NewInstanceService(repo ports.Repository, hypervisor ports.Hypervisor, dnsProvider ports.DnsProvider, logger ports.EventLogger, opts InstanceOptions) *InstanceService {
	return &InstanceService{
		repo:        repo,
		hypervisor:  hypervisor,
		dnsProvider: dnsProvider,
		logger:      logger,
		operator:    opts.Operator,
		deployer:    opts.Deployer,
		guestUser:   opts.GuestUser,
		sshKeyPath:  opts.SSHKeyPath,
	}
}

func (s *InstanceService) log(level, message string, fields map[string]any) {
	if s.logger != nil {
		s.logger.Append(level, "instance-service", message, fields)
	}
}

// ListInstances retorna todas las instancias registradas en el inventario,
// sincronizando su estado real de encendido con el hipervisor si está disponible.
func (s *InstanceService) ListInstances(ctx context.Context) ([]domain.Instance, error) {
	list, err := s.repo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	if running, ok := s.inspectRunningVMs(ctx); ok {
		for i := range list {
			if s.reconcilePowerState(&list[i], running) {
				_ = s.repo.Save(ctx, list[i])
			}
		}
	}
	return list, nil
}

// GetInstance busca una instancia por su identificador único y sincroniza su
// estado real de encendido con el hipervisor.
func (s *InstanceService) GetInstance(ctx context.Context, id string) (*domain.Instance, error) {
	inst, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if running, ok := s.inspectRunningVMs(ctx); ok {
		if s.reconcilePowerState(inst, running) {
			_ = s.repo.Save(ctx, *inst)
		}
	}
	return inst, nil
}

func (s *InstanceService) inspectRunningVMs(ctx context.Context) (map[string]bool, bool) {
	inspector, ok := s.hypervisor.(ports.PowerStateInspector)
	if !ok || inspector == nil {
		return nil, false
	}
	running, err := inspector.RunningVMs(ctx)
	if err != nil {
		return nil, false
	}
	return running, true
}

func (s *InstanceService) reconcilePowerState(inst *domain.Instance, running map[string]bool) bool {
	if inst == nil {
		return false
	}
	isRunning := running[inst.Name]
	if inst.Status == domain.StatusRunning && !isRunning {
		inst.Status = domain.StatusStopped
		return true
	}
	if inst.Status == domain.StatusStopped && isRunning {
		inst.Status = domain.StatusRunning
		return true
	}
	return false
}

// Activate enciende una instancia detenida, actualiza su IP y refresca el
// registro DNS si la dirección cambió.
func (s *InstanceService) Activate(ctx context.Context, id string) (*domain.Instance, error) {
	inst, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if running, ok := s.inspectRunningVMs(ctx); ok {
		if s.reconcilePowerState(inst, running) {
			_ = s.repo.Save(ctx, *inst)
		}
	}
	if inst.Status == domain.StatusRunning {
		return inst, nil
	}
	if inst.Status != domain.StatusStopped && inst.Status != domain.StatusError {
		return nil, fmt.Errorf("%w: la instancia '%s' está en estado %s", domain.ErrOperationNotAllowed, inst.Name, inst.Status)
	}
	if s.hypervisor == nil {
		return nil, fmt.Errorf("el hipervisor no está disponible")
	}

	s.log("INFO", "Activando instancia", map[string]any{"instancia": inst.Name})
	if err := s.hypervisor.Start(ctx, inst.Name); err != nil {
		return nil, err
	}
	ip, err := s.hypervisor.GetIPAddress(ctx, inst.Name)
	if err != nil {
		return nil, fmt.Errorf("fallo al obtener la IP de '%s': %w", inst.Name, err)
	}

	inst.IP = fmt.Sprintf("%s/24", ip.String())
	inst.Status = domain.StatusRunning
	if err := s.repo.Save(ctx, *inst); err != nil {
		return nil, err
	}

	if s.dnsProvider != nil {
		if zone := strings.TrimPrefix(inst.FQDN, inst.Name+"."); zone != "" {
			record, recordErr := domain.NewARecord(inst.Name, zone, ip, 300)
			if recordErr != nil {
				s.log("WARN", "No se pudo preparar el registro DNS al activar", map[string]any{"instancia": inst.Name, "error": recordErr.Error()})
			} else if err := s.dnsProvider.AddRecord(ctx, *record); err != nil {
				s.log("WARN", "No se pudo actualizar el DNS al activar", map[string]any{"instancia": inst.Name, "error": err.Error()})
			}
		}
	}

	s.log("INFO", "Instancia activada", map[string]any{"instancia": inst.Name, "ip": inst.IP})
	return inst, nil
}

// Deactivate apaga una instancia activa sin destruirla ni eliminar su DNS.
func (s *InstanceService) Deactivate(ctx context.Context, id string) (*domain.Instance, error) {
	inst, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if running, ok := s.inspectRunningVMs(ctx); ok {
		if s.reconcilePowerState(inst, running) {
			_ = s.repo.Save(ctx, *inst)
		}
	}
	if inst.Status == domain.StatusStopped {
		return inst, nil
	}
	if inst.Status != domain.StatusRunning {
		return nil, fmt.Errorf("%w: la instancia '%s' está en estado %s", domain.ErrOperationNotAllowed, inst.Name, inst.Status)
	}
	if s.hypervisor == nil {
		return nil, fmt.Errorf("el hipervisor no está disponible")
	}

	s.log("INFO", "Desactivando instancia", map[string]any{"instancia": inst.Name})
	if err := s.hypervisor.Stop(ctx, inst.Name); err != nil {
		return nil, err
	}
	inst.Status = domain.StatusStopped
	if err := s.repo.Save(ctx, *inst); err != nil {
		return nil, err
	}
	s.log("INFO", "Instancia desactivada", map[string]any{"instancia": inst.Name})
	return inst, nil
}

// RestartServices reinicia Apache en una instancia activa.
func (s *InstanceService) RestartServices(ctx context.Context, id string) (*domain.Instance, error) {
	inst, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	ip, err := runningIP(inst)
	if err != nil {
		return nil, err
	}
	if s.operator == nil {
		return nil, fmt.Errorf("el operador de instancias no está disponible")
	}

	s.log("INFO", "Reiniciando servicios de la instancia", map[string]any{"instancia": inst.Name, "ip": ip.String()})
	if err := s.operator.RestartWebService(ctx, inst.Name, ip); err != nil {
		return nil, err
	}
	s.log("INFO", "Servicios reiniciados correctamente", map[string]any{"instancia": inst.Name})
	return inst, nil
}

// Redeploy reemplaza el contenido web de una instancia activa sin recrear la VM
// ni el registro DNS.
func (s *InstanceService) Redeploy(ctx context.Context, id string, content io.ReaderAt, size int64) (*domain.Instance, error) {
	inst, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	ip, err := runningIP(inst)
	if err != nil {
		return nil, err
	}
	if s.deployer == nil {
		return nil, fmt.Errorf("el desplegador no está disponible")
	}
	if _, err := domain.ValidateZipPackage(content, size); err != nil {
		return nil, err
	}

	s.log("INFO", "Actualizando el despliegue de la instancia", map[string]any{"instancia": inst.Name, "bytes": size})
	stream := io.NewSectionReader(content, 0, size)
	if err := s.deployer.DeployWebZip(ctx, inst.Name, ip, stream, size); err != nil {
		return nil, err
	}
	s.log("INFO", "Despliegue actualizado correctamente", map[string]any{"instancia": inst.Name})
	return inst, nil
}

// GetAccess compone los datos y el comando SSH para acceder a la instancia.
func (s *InstanceService) GetAccess(ctx context.Context, id string) (domain.InstanceAccess, error) {
	inst, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return domain.InstanceAccess{}, err
	}
	ip, err := runningIP(inst)
	if err != nil {
		return domain.InstanceAccess{}, err
	}
	if s.sshKeyPath == "" {
		return domain.InstanceAccess{}, fmt.Errorf("no hay una clave SSH configurada para el entorno")
	}

	user := "debian"
	if s.guestUser != nil {
		if value := s.guestUser(); value != "" {
			user = value
		}
	}
	host := ip.String()
	command := fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=10 %s@%s", s.sshKeyPath, user, host)
	return domain.InstanceAccess{
		Usuario:             user,
		Host:                host,
		IP:                  host,
		FQDN:                inst.FQDN,
		Puerto:              22,
		Comando:             command,
		UsaClaveLaboratorio: true,
		NotaClave:           "El comando usa la clave privada del entorno en el anfitrión (" + s.sshKeyPath + "). No se expone su contenido.",
	}, nil
}

// DeleteInstance apaga y destruye la máquina virtual, remueve el registro DNS y elimina el registro del inventario.
func (s *InstanceService) DeleteInstance(ctx context.Context, id string) error {
	inst, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	log.Printf("[InstanceService] Eliminando instancia '%s'", inst.Name)
	s.log("INFO", "Eliminando instancia", map[string]any{"instancia": inst.Name})

	// 1. Apagar y destruir VM en VirtualBox
	_ = s.hypervisor.Stop(ctx, inst.Name)
	if err := s.hypervisor.Destroy(ctx, inst.Name); err != nil {
		log.Printf("[InstanceService] Advertencia al destruir VM '%s': %v", inst.Name, err)
	}

	// 2. Dar de baja registro DNS en Bind9
	if err := s.dnsProvider.RemoveRecord(ctx, inst.Name); err != nil {
		log.Printf("[InstanceService] Advertencia al remover registro DNS '%s': %v", inst.Name, err)
	}

	// 3. Eliminar de la persistencia
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("fallo al eliminar instancia de inventario: %w", err)
	}

	log.Printf("[InstanceService] Instancia '%s' eliminada exitosamente", inst.Name)
	s.log("INFO", "Instancia eliminada exitosamente", map[string]any{"instancia": inst.Name})
	return nil
}

// runningIP valida que la instancia esté activa y retorna su IP utilizable.
func runningIP(inst *domain.Instance) (netip.Addr, error) {
	if inst == nil {
		return netip.Addr{}, domain.ErrInstanceNotFound
	}
	if inst.Status != domain.StatusRunning {
		return netip.Addr{}, fmt.Errorf("%w: la instancia '%s' está en estado %s", domain.ErrOperationNotAllowed, inst.Name, inst.Status)
	}
	raw := strings.TrimSpace(strings.Split(inst.IP, "/")[0])
	ip, err := netip.ParseAddr(raw)
	if err != nil || !ip.IsValid() {
		return netip.Addr{}, fmt.Errorf("%w: la instancia '%s' no tiene una IP válida", domain.ErrOperationNotAllowed, inst.Name)
	}
	return ip, nil
}
