// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

// Package services implementa los casos de uso y la lógica de orquestación transaccional del sistema HTTPaaS.
package services

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/netip"
	"strings"
	"time"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/domain"
	"github.com/Andrey3681/httpaas-orchestrator/internal/core/ports"
)

// ProvisionRequest encapsula los datos requeridos para solicitar un nuevo servidor HTTP.
type ProvisionRequest struct {
	Hostname   string
	ZipContent io.ReaderAt
	ZipSize    int64
	ZipStream  io.Reader
}

// Orchestrator coordina el ciclo de vida del aprovisionamiento desatendido con soporte de rollback.
type Orchestrator struct {
	hypervisor   ports.Hypervisor
	dnsProvider  ports.DnsProvider
	deployer     ports.Deployer
	repo         ports.Repository
	domainZone   string
	baseDiskPath func() string
	logger       ports.EventLogger
}

// NewOrchestrator instancia el orquestador inyectando sus puertos colaboradores.
func NewOrchestrator(
	hypervisor ports.Hypervisor,
	dnsProvider ports.DnsProvider,
	deployer ports.Deployer,
	repo ports.Repository,
	domainZone string,
	baseDiskPath func() string,
	logger ports.EventLogger,
) *Orchestrator {
	return &Orchestrator{
		hypervisor:   hypervisor,
		dnsProvider:  dnsProvider,
		deployer:     deployer,
		repo:         repo,
		domainZone:   domainZone,
		baseDiskPath: baseDiskPath,
		logger:       logger,
	}
}

func (o *Orchestrator) log(level, message string, fields map[string]any) {
	if o.logger != nil {
		o.logger.Append(level, "orchestrator", message, fields)
	}
}

// Provision ejecuta el flujo completo de creación de instancia, configuración de red, DNS y despliegue.
func (o *Orchestrator) Provision(ctx context.Context, req ProvisionRequest) (*domain.Instance, error) {
	// 1. Validaciones de dominio previas
	if err := domain.ValidateHostname(req.Hostname); err != nil {
		return nil, err
	}

	trimmedName := strings.ToLower(strings.TrimSpace(req.Hostname))

	// 2. Verificar que no exista en el inventario
	if existing, _ := o.repo.FindByID(ctx, trimmedName); existing != nil {
		return nil, fmt.Errorf("%w: el nombre '%s' ya está en uso", domain.ErrHostAlreadyExists, trimmedName)
	}

	// 3. Validar seguridad del paquete .zip (Zip Slip / tamaño)
	if _, err := domain.ValidateZipPackage(req.ZipContent, req.ZipSize); err != nil {
		return nil, err
	}

	log.Printf("[Orchestrator] Iniciando aprovisionamiento para host '%s'", trimmedName)
	o.log("INFO", "Iniciando aprovisionamiento de instancia", map[string]any{"instancia": trimmedName})

	// 4. Crear máquina virtual a partir del disco multiconexión base
	baseDisk := ""
	if o.baseDiskPath != nil {
		baseDisk = o.baseDiskPath()
	}
	if baseDisk == "" {
		return nil, fmt.Errorf("no hay disco base disponible; completa la preparación inicial")
	}
	inst, err := o.hypervisor.CreateInstanceFromBase(ctx, trimmedName, baseDisk)
	if err != nil {
		return nil, fmt.Errorf("fallo al crear máquina virtual en VirtualBox: %w", err)
	}

	vmCreated := true
	dnsRegistered := false

	// Rutina de compensación en diferido (Rollback) si ocurre algún error posterior
	defer func() {
		if err != nil {
			log.Printf("[Rollback] Iniciando compensación tras fallo en host '%s': %v", trimmedName, err)
			o.log("WARN", "Iniciando rollback de aprovisionamiento", map[string]any{"instancia": trimmedName, "error": err.Error()})
			if dnsRegistered {
				_ = o.dnsProvider.RemoveRecord(context.Background(), trimmedName)
			}
			if vmCreated {
				_ = o.hypervisor.Stop(context.Background(), trimmedName)
				_ = o.hypervisor.Destroy(context.Background(), trimmedName)
			}
		}
	}()

	// 5. Iniciar la máquina virtual
	if err = o.hypervisor.Start(ctx, trimmedName); err != nil {
		return nil, fmt.Errorf("fallo al iniciar máquina virtual '%s': %w", trimmedName, err)
	}

	// 6. Obtener dirección IP asignada
	var ip netip.Addr
	ip, err = o.hypervisor.GetIPAddress(ctx, trimmedName)
	if err != nil {
		return nil, fmt.Errorf("fallo al obtener dirección IP de '%s': %w", trimmedName, err)
	}

	inst.IP = fmt.Sprintf("%s/24", ip.String())
	o.log("INFO", "Dirección IP obtenida para la instancia", map[string]any{"instancia": trimmedName, "ip": inst.IP})

	// 7. Registrar en servidor DNS Bind9
	var record *domain.DnsRecord
	record, err = domain.NewARecord(trimmedName, o.domainZone, ip, 300)
	if err != nil {
		return nil, err
	}

	if err = o.dnsProvider.AddRecord(ctx, *record); err != nil {
		return nil, fmt.Errorf("fallo al registrar DNS en Bind9: %w", err)
	}
	dnsRegistered = true
	o.log("INFO", "Registro DNS creado para la instancia", map[string]any{"instancia": trimmedName, "fqdn": inst.FQDN})

	// 8. Desplegar paquete web .zip en Apache
	if err = o.deployer.DeployWebZip(ctx, trimmedName, ip, req.ZipStream, req.ZipSize); err != nil {
		return nil, fmt.Errorf("fallo al desplegar archivos web en Apache: %w", err)
	}

	// 9. Actualizar estado y persistir en repositorio
	inst.Status = domain.StatusRunning
	inst.CreatedAt = time.Now().UTC()

	if err = o.repo.Save(ctx, *inst); err != nil {
		return nil, fmt.Errorf("fallo al guardar instancia en inventario: %w", err)
	}

	log.Printf("[Orchestrator] Instancia '%s' aprovisionada con éxito (IP: %s, FQDN: %s)",
		inst.Name, inst.IP, inst.FQDN)
	o.log("INFO", "Instancia aprovisionada con éxito", map[string]any{"instancia": inst.Name, "ip": inst.IP, "fqdn": inst.FQDN})

	return inst, nil
}
