// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package handlers_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/domain"
	"github.com/Andrey3681/httpaas-orchestrator/internal/handlers"
	"github.com/Andrey3681/httpaas-orchestrator/internal/services"
)

type dummyHypervisor struct{}

func (d *dummyHypervisor) CreateInstanceFromBase(ctx context.Context, name string, baseDisk string) (*domain.Instance, error) {
	return domain.NewInstance(name, "cloud.local")
}
func (d *dummyHypervisor) Start(ctx context.Context, name string) error { return nil }
func (d *dummyHypervisor) GetIPAddress(ctx context.Context, name string) (netip.Addr, error) {
	return netip.MustParseAddr("192.168.10.30"), nil
}
func (d *dummyHypervisor) Stop(ctx context.Context, name string) error    { return nil }
func (d *dummyHypervisor) Destroy(ctx context.Context, name string) error { return nil }

type dummyDnsProvider struct{}

func (d *dummyDnsProvider) AddRecord(ctx context.Context, record domain.DnsRecord) error { return nil }

func (d *dummyDnsProvider) RemoveRecord(ctx context.Context, hostname string) error { return nil }

func (d *dummyDnsProvider) Resolve(ctx context.Context, hostname string) (netip.Addr, error) {
	return netip.MustParseAddr("192.168.10.30"), nil
}

func (d *dummyDnsProvider) ListRecords(ctx context.Context) ([]domain.DnsRecord, error) {
	return nil, nil
}

type dummyDeployer struct{}

func (d *dummyDeployer) DeployWebZip(ctx context.Context, hostname string, targetIP netip.Addr, zipContent io.Reader, size int64) error {
	return nil
}

type inMemoryRepo struct {
	instances  map[string]domain.Instance
	findAllErr error
}

func (m *inMemoryRepo) Save(ctx context.Context, inst domain.Instance) error {
	m.instances[inst.ID] = inst
	return nil
}

func (m *inMemoryRepo) FindByID(ctx context.Context, id string) (*domain.Instance, error) {
	inst, ok := m.instances[id]
	if !ok {
		return nil, domain.ErrInstanceNotFound
	}
	return &inst, nil
}

func (m *inMemoryRepo) FindAll(ctx context.Context) ([]domain.Instance, error) {
	if m.findAllErr != nil {
		return nil, m.findAllErr
	}
	var list []domain.Instance
	for _, v := range m.instances {
		list = append(list, v)
	}
	return list, nil
}

func (m *inMemoryRepo) Delete(ctx context.Context, id string) error {
	delete(m.instances, id)
	return nil
}

func TestInstanceHandler_List(t *testing.T) {
	repo := &inMemoryRepo{
		instances: map[string]domain.Instance{
			"paisajes": {
				ID:        "paisajes",
				Name:      "paisajes",
				FQDN:      "paisajes.cloud.local",
				IP:        "192.168.10.30/24",
				URL:       "http://paisajes.cloud.local",
				Status:    domain.StatusRunning,
				CreatedAt: time.Now().UTC(),
			},
		},
	}

	hyp := &dummyHypervisor{}
	dns := &dummyDnsProvider{}
	dep := &dummyDeployer{}

	orc := services.NewOrchestrator(hyp, dns, dep, repo, "cloud.local", func() string { return "ApacheServer.vdi" }, nil)
	svc := services.NewInstanceService(repo, hyp, dns, nil, services.InstanceOptions{})
	handler := handlers.NewInstanceHandler(orc, svc, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/instancias", nil)
	rec := httptest.NewRecorder()

	handler.HandleInstances(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestInstanceHandler_GetNotFound(t *testing.T) {
	repo := &inMemoryRepo{instances: make(map[string]domain.Instance)}
	hyp := &dummyHypervisor{}
	dns := &dummyDnsProvider{}
	dep := &dummyDeployer{}

	orc := services.NewOrchestrator(hyp, dns, dep, repo, "cloud.local", func() string { return "ApacheServer.vdi" }, nil)
	svc := services.NewInstanceService(repo, hyp, dns, nil, services.InstanceOptions{})
	handler := handlers.NewInstanceHandler(orc, svc, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/instancias/no-existe", nil)
	rec := httptest.NewRecorder()

	handler.HandleInstance(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestInstanceHandler_ListInternalError(t *testing.T) {
	repo := &inMemoryRepo{instances: make(map[string]domain.Instance), findAllErr: errors.New("disk failure")}
	hyp := &dummyHypervisor{}
	dns := &dummyDnsProvider{}
	dep := &dummyDeployer{}

	orc := services.NewOrchestrator(hyp, dns, dep, repo, "cloud.local", func() string { return "ApacheServer.vdi" }, nil)
	svc := services.NewInstanceService(repo, hyp, dns, nil, services.InstanceOptions{})
	handler := handlers.NewInstanceHandler(orc, svc, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/instancias", nil)
	rec := httptest.NewRecorder()

	handler.HandleInstances(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "disk failure") {
		t.Fatalf("expected sanitized 500 message, got %q", body)
	}
	if !strings.Contains(body, "Error interno") {
		t.Fatalf("expected generic 500 message, got %q", body)
	}
}
