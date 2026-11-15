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

type actionRepo struct {
	instance domain.Instance
	err      error
}

func (r *actionRepo) Save(_ context.Context, inst domain.Instance) error {
	r.instance = inst
	return nil
}
func (r *actionRepo) FindByID(_ context.Context, id string) (*domain.Instance, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.instance.ID != id {
		return nil, domain.ErrInstanceNotFound
	}
	copy := r.instance
	return &copy, nil
}
func (r *actionRepo) FindAll(context.Context) ([]domain.Instance, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.instance.ID == "" {
		return nil, nil
	}
	return []domain.Instance{r.instance}, nil
}
func (r *actionRepo) Delete(context.Context, string) error { return nil }

type powerHypervisor struct {
	ip         netip.Addr
	started    bool
	stopped    bool
	err        error
	runningVMs map[string]bool
}

func (f *powerHypervisor) RunningVMs(context.Context) (map[string]bool, error) {
	if f.runningVMs == nil {
		return nil, errors.New("inspection not configured")
	}
	return f.runningVMs, nil
}

func (f *powerHypervisor) CreateInstanceFromBase(context.Context, string, string) (*domain.Instance, error) {
	return nil, nil
}

func (f *powerHypervisor) Start(context.Context, string) error {
	f.started = true
	return f.err
}

func (f *powerHypervisor) GetIPAddress(context.Context, string) (netip.Addr, error) {
	if f.err != nil {
		return netip.Addr{}, f.err
	}
	return f.ip, nil
}

func (f *powerHypervisor) Stop(context.Context, string) error {
	f.stopped = true
	return f.err
}
func (f *powerHypervisor) Destroy(context.Context, string) error { return nil }

type fakeOperator struct {
	calls int
	err   error
}

func (f *fakeOperator) RestartWebService(context.Context, string, netip.Addr) error {
	f.calls++
	return f.err
}

type fakeActionDeployer struct {
	calls int
	err   error
}

func (f *fakeActionDeployer) DeployWebZip(context.Context, string, netip.Addr, io.Reader, int64) error {
	f.calls++
	return f.err
}

func runningInstance() domain.Instance {
	return domain.Instance{
		ID:     "mario",
		Name:   "mario",
		FQDN:   "mario.cloud.local",
		IP:     "192.168.57.105/24",
		Status: domain.StatusRunning,
	}
}

func newActionService(inst domain.Instance, op *fakeOperator, dep *fakeActionDeployer) *services.InstanceService {
	return services.NewInstanceService(
		&actionRepo{instance: inst},
		nil, nil, nil,
		services.InstanceOptions{
			Operator:   op,
			Deployer:   dep,
			GuestUser:  func() string { return "jorge" },
			SSHKeyPath: "/keys/id_ed25519",
		},
	)
}

func TestInstanceService_Activate(t *testing.T) {
	repo := &actionRepo{instance: domain.Instance{
		ID: "mario", Name: "mario", FQDN: "mario.cloud.local",
		IP: "192.168.57.105/24", Status: domain.StatusStopped,
	}}
	hyp := &powerHypervisor{ip: netip.MustParseAddr("192.168.57.106")}
	svc := services.NewInstanceService(repo, hyp, nil, nil, services.InstanceOptions{})

	inst, err := svc.Activate(context.Background(), "mario")
	if err != nil {
		t.Fatalf("activate: %v", err)
	}
	if !hyp.started {
		t.Fatal("expected hypervisor.Start to be called")
	}
	if inst.Status != domain.StatusRunning || inst.IP != "192.168.57.106/24" {
		t.Fatalf("unexpected instance after activate: %+v", inst)
	}
}

func TestInstanceService_Deactivate(t *testing.T) {
	repo := &actionRepo{instance: runningInstance()}
	hyp := &powerHypervisor{ip: netip.MustParseAddr("192.168.57.105")}
	svc := services.NewInstanceService(repo, hyp, nil, nil, services.InstanceOptions{})

	inst, err := svc.Deactivate(context.Background(), "mario")
	if err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if !hyp.stopped {
		t.Fatal("expected hypervisor.Stop to be called")
	}
	if inst.Status != domain.StatusStopped {
		t.Fatalf("expected DETENIDA, got %s", inst.Status)
	}
}

func TestInstanceService_ActivateRejectsCreating(t *testing.T) {
	repo := &actionRepo{instance: domain.Instance{ID: "mario", Name: "mario", Status: domain.StatusCreating}}
	svc := services.NewInstanceService(repo, &powerHypervisor{}, nil, nil, services.InstanceOptions{})
	if _, err := svc.Activate(context.Background(), "mario"); err == nil {
		t.Fatal("expected error for CREANDO")
	}
}

func TestInstanceService_DeactivateRejectsCreating(t *testing.T) {
	repo := &actionRepo{instance: domain.Instance{ID: "mario", Name: "mario", Status: domain.StatusCreating}}
	svc := services.NewInstanceService(repo, &powerHypervisor{}, nil, nil, services.InstanceOptions{})
	if _, err := svc.Deactivate(context.Background(), "mario"); err == nil {
		t.Fatal("expected error for CREANDO")
	}
}

func TestInstanceService_ListInstancesDetectsPoweredOffVM(t *testing.T) {
	repo := &actionRepo{instance: runningInstance()}
	hyp := &powerHypervisor{
		ip:         netip.MustParseAddr("192.168.57.105"),
		runningVMs: map[string]bool{},
	}
	svc := services.NewInstanceService(repo, hyp, nil, nil, services.InstanceOptions{})

	list, err := svc.ListInstances(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 1 || list[0].Status != domain.StatusStopped {
		t.Fatalf("expected instance to be detected as DETENIDA, got: %+v", list)
	}
	if repo.instance.Status != domain.StatusStopped {
		t.Errorf("expected repository status to be updated to DETENIDA, got %s", repo.instance.Status)
	}
}

func TestInstanceService_ActivateWorksWhenVMWasExternallyPoweredOff(t *testing.T) {
	repo := &actionRepo{instance: runningInstance()}
	hyp := &powerHypervisor{
		ip:         netip.MustParseAddr("192.168.57.108"),
		runningVMs: map[string]bool{},
	}
	svc := services.NewInstanceService(repo, hyp, nil, nil, services.InstanceOptions{})

	inst, err := svc.Activate(context.Background(), "mario")
	if err != nil {
		t.Fatalf("unexpected error activating externally powered-off VM: %v", err)
	}
	if !hyp.started {
		t.Fatal("expected hypervisor.Start to be called when VM was externally powered off")
	}
	if inst.Status != domain.StatusRunning || inst.IP != "192.168.57.108/24" {
		t.Fatalf("unexpected instance after activate: %+v", inst)
	}
}

func TestInstanceService_RestartServices(t *testing.T) {
	op := &fakeOperator{}
	svc := newActionService(runningInstance(), op, &fakeActionDeployer{})

	inst, err := svc.RestartServices(context.Background(), "mario")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inst.Name != "mario" || op.calls != 1 {
		t.Errorf("unexpected restart: inst=%v calls=%d", inst.Name, op.calls)
	}
}

func TestInstanceService_RestartServicesRejectsInactive(t *testing.T) {
	inst := runningInstance()
	inst.Status = domain.StatusError
	svc := newActionService(inst, &fakeOperator{}, &fakeActionDeployer{})

	_, err := svc.RestartServices(context.Background(), "mario")
	if !errors.Is(err, domain.ErrOperationNotAllowed) {
		t.Fatalf("expected ErrOperationNotAllowed, got %v", err)
	}
}

func TestInstanceService_RestartServicesUnknownInstance(t *testing.T) {
	svc := newActionService(runningInstance(), &fakeOperator{}, &fakeActionDeployer{})
	if _, err := svc.RestartServices(context.Background(), "otro"); !errors.Is(err, domain.ErrInstanceNotFound) {
		t.Fatalf("expected ErrInstanceNotFound, got %v", err)
	}
}

func TestInstanceService_Redeploy(t *testing.T) {
	dep := &fakeActionDeployer{}
	svc := newActionService(runningInstance(), &fakeOperator{}, dep)

	content := zipWithIndex(t)
	inst, err := svc.Redeploy(context.Background(), "mario", bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inst.Name != "mario" || dep.calls != 1 {
		t.Errorf("unexpected redeploy: inst=%v calls=%d", inst.Name, dep.calls)
	}
}

func TestInstanceService_RedeployRejectsInvalidZip(t *testing.T) {
	svc := newActionService(runningInstance(), &fakeOperator{}, &fakeActionDeployer{})
	data := []byte("no soy un zip")
	_, err := svc.Redeploy(context.Background(), "mario", bytes.NewReader(data), int64(len(data)))
	if !errors.Is(err, domain.ErrInvalidWebPackage) {
		t.Fatalf("expected ErrInvalidWebPackage, got %v", err)
	}
}

func TestInstanceService_GetAccess(t *testing.T) {
	svc := newActionService(runningInstance(), &fakeOperator{}, &fakeActionDeployer{})

	access, err := svc.GetAccess(context.Background(), "mario")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if access.Usuario != "jorge" || access.Host != "192.168.57.105" || access.Puerto != 22 {
		t.Errorf("unexpected access: %+v", access)
	}
	if access.FQDN != "mario.cloud.local" || access.Comando == "" || !access.UsaClaveLaboratorio {
		t.Errorf("incomplete access: %+v", access)
	}
}

func zipWithIndex(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	file, err := w.Create("index.html")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("<h1>ok</h1>")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
