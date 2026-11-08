// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package filestore_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Andrey3681/httpaas-orchestrator/internal/adapters/filestore"
	"github.com/Andrey3681/httpaas-orchestrator/internal/core/domain"
)

func TestJSONRepository_CRUD(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "instances.json")

	repo, err := filestore.NewJSONRepository(filePath)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	ctx := context.Background()

	// 1. Initial should be empty
	all, err := repo.FindAll(ctx)
	if err != nil {
		t.Fatalf("unexpected error finding all: %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("expected 0 items, got %d", len(all))
	}

	// 2. Save instance
	inst := domain.Instance{
		ID:        "paisajes",
		Name:      "paisajes",
		FQDN:      "paisajes.cloud.local",
		IP:        "192.168.10.30/24",
		URL:       "http://paisajes.cloud.local",
		Status:    domain.StatusRunning,
		CreatedAt: time.Now().UTC(),
	}

	err = repo.Save(ctx, inst)
	if err != nil {
		t.Fatalf("unexpected error saving instance: %v", err)
	}

	// 3. Find by ID
	found, err := repo.FindByID(ctx, "paisajes")
	if err != nil {
		t.Fatalf("unexpected error finding by ID: %v", err)
	}
	if found.Name != "paisajes" {
		t.Errorf("expected name 'paisajes', got '%s'", found.Name)
	}

	// 4. Delete
	err = repo.Delete(ctx, "paisajes")
	if err != nil {
		t.Fatalf("unexpected error deleting: %v", err)
	}

	// 5. Verify deleted
	_, err = repo.FindByID(ctx, "paisajes")
	if err == nil {
		t.Fatalf("expected error finding deleted instance, got nil")
	}
}
