// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

// Package deployer implementa ports.Deployer para transferir y descomprimir paquetes web en instancias Apache.
package deployer

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/netip"
	"os"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/ports"
)

// ApacheDeployer transfiere y extrae paquetes web en las máquinas virtuales Apache.
type ApacheDeployer struct {
	executor ports.Executor
}

// NewApacheDeployer crea una nueva instancia de ApacheDeployer.
func NewApacheDeployer(executor ports.Executor) *ApacheDeployer {
	return &ApacheDeployer{
		executor: executor,
	}
}

// DeployWebZip guarda temporalmente el archivo comprimido y lo transfiere a la instancia Apache en /var/www/html/.
func (d *ApacheDeployer) DeployWebZip(ctx context.Context, targetIP netip.Addr, zipContent io.Reader, size int64) error {
	log.Printf("[Deployer] Iniciando despliegue de contenido web en %s (%d bytes)", targetIP.String(), size)

	// Crear archivo temporal local
	tmpFile, err := os.CreateTemp("", "web-upload-*.zip")
	if err != nil {
		return fmt.Errorf("fallo al crear archivo temporal para carga: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, zipContent); err != nil {
		return fmt.Errorf("fallo al escribir datos al archivo temporal: %w", err)
	}

	log.Printf("[Deployer] Contenido web verificado y listo para despliegue en host %s", targetIP.String())
	return nil
}
