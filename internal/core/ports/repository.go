// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package ports

import (
	"context"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/domain"
)

// Repository administra la persistencia del inventario de instancias aprovisionadas.
type Repository interface {
	// Save almacena o actualiza la instancia en el almacenamiento.
	Save(ctx context.Context, instance domain.Instance) error

	// FindByID recupera una instancia por su identificador único o nombre.
	FindByID(ctx context.Context, id string) (*domain.Instance, error)

	// FindAll retorna la lista completa de instancias activas.
	FindAll(ctx context.Context) ([]domain.Instance, error)

	// Delete elimina la instancia del repositorio de datos.
	Delete(ctx context.Context, id string) error
}
