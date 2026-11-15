// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package services_test

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"net/netip"
	"testing"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/domain"
	"github.com/Andrey3681/httpaas-orchestrator/internal/services"
)

// Dobles de prueba (Fakes)
type fakeHypervisor struct {
	createdVMs   []string
	startedVMs   []string
	destroyedVMs []string
	ipToReturn   netip.Addr
	failCreate   bool
}

func (f *fakeHypervisor) CreateInstanceFromBase(ctx context.Context, name string, baseDisk string) (*domain.Instance, error) {
	if f.failCreate {
		return nil, errors.New("simulated vbox create error")
	}
	f.createdVMs = append(f.createdVMs, name)
	return domain.NewInstance(name, "cloud.local")
}

func (f *fakeHypervisor) Start(ctx context.Context, name string) error {
	f.startedVMs = append(f.startedVMs, name)
	return nil
}

func (f *fakeHypervisor) GetIPAddress(ctx context.Context, name string) (netip.Addr, error) {
	return f.ipToReturn, nil
}

func (f *fakeHypervisor) Stop(ctx context.Context, name string) error {
	return nil
}

func (f *fakeHypervisor) Destroy(ctx context.Context, name string) error {
	f.destroyedVMs = append(f.destroyedVMs, name)
	return nil
}

type fakeDnsProvider struct {
	records      map[string]domain.DnsRecord
	failAdd      bool
	removedHosts []string
}

func (f *fakeDnsProvider) AddRecord(ctx context.Context, record domain.DnsRecord) error {
	if f.failAdd {
		return errors.New("simulated dns error")
	}
	f.records[record.Name] = record
	return nil
}

func (f *fakeDnsProvider) RemoveRecord(ctx context.Context, hostname string) error {
	f.removedHosts = append(f.removedHosts, hostname)
	delete(f.records, hostname)
	return nil
}

func (f *fakeDnsProvider) Resolve(ctx context.Context, hostname string) (netip.Addr, error) {
	return netip.MustParseAddr("192.168.10.30"), nil
}

func (f *fakeDnsProvider) ListRecords(ctx context.Context) ([]domain.DnsRecord, error) {
	return nil, nil
}

type fakeDeployer struct {
	deployedIPs []string
	failDeploy  bool
}

func (f *fakeDeployer) DeployWebZip(_ context.Context, hostname string, targetIP netip.Addr, _ io.Reader, _ int64) error {
	if f.failDeploy {
		return errors.New("simulated deployer error")
	}
	f.deployedIPs = append(f.deployedIPs, targetIP.String())
	return nil
}

type fakeRepository struct {
	instances map[string]domain.Instance
}

func (f *fakeRepository) Save(ctx context.Context, inst domain.Instance) error {
	f.instances[inst.ID] = inst
	return nil
}

func (f *fakeRepository) FindByID(ctx context.Context, id string) (*domain.Instance, error) {
	inst, ok := f.instances[id]
	if !ok {
		return nil, domain.ErrInstanceNotFound
	}
	return &inst, nil
}

func (f *fakeRepository) FindAll(ctx context.Context) ([]domain.Instance, error) {
	res := make([]domain.Instance, 0, len(f.instances))
	for _, v := range f.instances {
		res = append(res, v)
	}
	return res, nil
}

func (f *fakeRepository) Delete(ctx context.Context, id string) error {
	delete(f.instances, id)
	return nil
}

func createTestZip(t *testing.T, filename, content string) ([]byte, int64) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, err := zw.Create(filename)
	if err != nil {
		t.Fatalf("failed to create zip: %v", err)
	}
	_, _ = f.Write([]byte(content))
	_ = zw.Close()
	return buf.Bytes(), int64(buf.Len())
}

func TestOrchestrator_Provision_Success(t *testing.T) {
	hyp := &fakeHypervisor{ipToReturn: netip.MustParseAddr("192.168.10.30")}
	dns := &fakeDnsProvider{records: make(map[string]domain.DnsRecord)}
	dep := &fakeDeployer{}
	repo := &fakeRepository{instances: make(map[string]domain.Instance)}

	orc := services.NewOrchestrator(hyp, dns, dep, repo, "cloud.local", func() string { return "ApacheServer.vdi" }, nil)

	zipBytes, size := createTestZip(t, "index.html", "<h1>Bienvenido</h1>")
	req := services.ProvisionRequest{
		Hostname:   "paisajes",
		ZipContent: bytes.NewReader(zipBytes),
		ZipSize:    size,
		ZipStream:  bytes.NewReader(zipBytes),
	}

	inst, err := orc.Provision(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error provisioning: %v", err)
	}

	if inst.Name != "paisajes" {
		t.Errorf("expected name 'paisajes', got '%s'", inst.Name)
	}
	if inst.Status != domain.StatusRunning {
		t.Errorf("expected status ACTIVA, got '%s'", inst.Status)
	}
	if len(hyp.createdVMs) != 1 || hyp.createdVMs[0] != "paisajes" {
		t.Errorf("VM was not created in hypervisor")
	}
	if len(dep.deployedIPs) != 1 || dep.deployedIPs[0] != "192.168.10.30" {
		t.Errorf("Zip was not deployed to target IP")
	}
}

func TestOrchestrator_Provision_RollbackOnDeployFailure(t *testing.T) {
	hyp := &fakeHypervisor{ipToReturn: netip.MustParseAddr("192.168.10.30")}
	dns := &fakeDnsProvider{records: make(map[string]domain.DnsRecord)}
	dep := &fakeDeployer{failDeploy: true} // Simular fallo de despliegue
	repo := &fakeRepository{instances: make(map[string]domain.Instance)}

	orc := services.NewOrchestrator(hyp, dns, dep, repo, "cloud.local", func() string { return "ApacheServer.vdi" }, nil)

	zipBytes, size := createTestZip(t, "index.html", "<h1>Bienvenido</h1>")
	req := services.ProvisionRequest{
		Hostname:   "paisajes",
		ZipContent: bytes.NewReader(zipBytes),
		ZipSize:    size,
		ZipStream:  bytes.NewReader(zipBytes),
	}

	_, err := orc.Provision(context.Background(), req)
	if err == nil {
		t.Fatalf("expected error from failed deployer, got nil")
	}

	// Comprobar rollback
	if len(hyp.destroyedVMs) != 1 || hyp.destroyedVMs[0] != "paisajes" {
		t.Errorf("expected VM to be destroyed during rollback")
	}
	if len(dns.removedHosts) != 1 || dns.removedHosts[0] != "paisajes" {
		t.Errorf("expected DNS record to be removed during rollback")
	}
}
