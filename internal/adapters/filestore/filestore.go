// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

// Package filestore implementa el puerto ports.Repository persistiendo el inventario de instancias en un archivo JSON atómico.
package filestore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/domain"
	"github.com/Andrey3681/httpaas-orchestrator/internal/core/ports"
)

// JSONRepository persiste el inventario en un archivo JSON con sincronización de hilos.
type JSONRepository struct {
	mu       sync.RWMutex
	filePath string
}

// NewJSONRepository inicializa el repositorio asegurando que el archivo y directorio existan.
func NewJSONRepository(filePath string) (ports.Repository, error) {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("fallo al crear directorio para repositorio: %w", err)
	}

	repo := &JSONRepository{
		filePath: filePath,
	}

	// Si no existe, inicializar con lista vacía
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		if err := repo.writeLocked([]domain.Instance{}); err != nil {
			return nil, err
		}
	}

	return repo, nil
}

// Save almacena o actualiza una instancia en el inventario.
func (r *JSONRepository) Save(ctx context.Context, instance domain.Instance) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	items, err := r.readLocked()
	if err != nil {
		return err
	}

	updated := false
	for i, item := range items {
		if item.ID == instance.ID || item.Name == instance.Name {
			items[i] = instance
			updated = true
			break
		}
	}

	if !updated {
		items = append(items, instance)
	}

	return r.writeLocked(items)
}

// FindByID busca una instancia por su identificador único o nombre.
func (r *JSONRepository) FindByID(ctx context.Context, id string) (*domain.Instance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items, err := r.readLocked()
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		if item.ID == id || item.Name == id {
			copy := item
			return &copy, nil
		}
	}

	return nil, domain.ErrInstanceNotFound
}

// FindAll retorna todas las instancias registradas.
func (r *JSONRepository) FindAll(ctx context.Context) ([]domain.Instance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.readLocked()
}

// Delete remueve una instancia del inventario.
func (r *JSONRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	items, err := r.readLocked()
	if err != nil {
		return err
	}

	index := -1
	for i, item := range items {
		if item.ID == id || item.Name == id {
			index = i
			break
		}
	}

	if index == -1 {
		return domain.ErrInstanceNotFound
	}

	items = append(items[:index], items[index+1:]...)
	return r.writeLocked(items)
}

func (r *JSONRepository) readLocked() ([]domain.Instance, error) {
	data, err := os.ReadFile(r.filePath)
	if err != nil {
		return nil, fmt.Errorf("fallo al leer archivo de instancias: %w", err)
	}

	var items []domain.Instance
	if len(data) == 0 {
		return items, nil
	}

	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("fallo al decodificar JSON de instancias: %w", err)
	}

	return items, nil
}

func (r *JSONRepository) writeLocked(items []domain.Instance) error {
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return fmt.Errorf("fallo al serializar instancias a JSON: %w", err)
	}

	tmpFile := r.filePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0o644); err != nil {
		return fmt.Errorf("fallo al escribir archivo temporal: %w", err)
	}

	if err := os.Rename(tmpFile, r.filePath); err != nil {
		return fmt.Errorf("fallo al renombrar archivo temporal: %w", err)
	}

	return nil
}
