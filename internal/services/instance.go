// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package services

import (
	"context"
	"fmt"
	"log"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/domain"
	"github.com/Andrey3681/httpaas-orchestrator/internal/core/ports"
)

// InstanceService gestiona las operaciones de consulta y eliminación controlada de instancias.
type InstanceService struct {
	repo        ports.Repository
	hypervisor  ports.Hypervisor
	dnsProvider ports.DnsProvider
	logger      ports.EventLogger
}

// NewInstanceService crea una nueva instancia del servicio de gestión.
func NewInstanceService(repo ports.Repository, hypervisor ports.Hypervisor, dnsProvider ports.DnsProvider, logger ports.EventLogger) *InstanceService {
	return &InstanceService{
		repo:        repo,
		hypervisor:  hypervisor,
		dnsProvider: dnsProvider,
		logger:      logger,
	}
}

func (s *InstanceService) log(level, message string, fields map[string]any) {
	if s.logger != nil {
		s.logger.Append(level, "instance-service", message, fields)
	}
}

// ListInstances retorna todas las instancias registradas en el inventario.
func (s *InstanceService) ListInstances(ctx context.Context) ([]domain.Instance, error) {
	return s.repo.FindAll(ctx)
}

// GetInstance busca una instancia por su identificador único.
func (s *InstanceService) GetInstance(ctx context.Context, id string) (*domain.Instance, error) {
	return s.repo.FindByID(ctx, id)
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
