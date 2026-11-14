// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package services_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/domain"
	"github.com/Andrey3681/httpaas-orchestrator/internal/core/ports"
	"github.com/Andrey3681/httpaas-orchestrator/internal/services"
)

type fakeProvisioner struct {
	mu         sync.Mutex
	inspect    domain.EnvironmentStatus
	inspectErr error
	ensureErr  error
	createErr  error
	reserveErr error
	waitErr    error
	scriptErr  error
	multiErr   error
	baseDisk   string
	calls      []string
	block      chan struct{}
}

func (f *fakeProvisioner) record(name string) {
	f.mu.Lock()
	f.calls = append(f.calls, name)
	f.mu.Unlock()
}

func (f *fakeProvisioner) Inspect(_ context.Context, _ domain.SetupConfig) (domain.EnvironmentStatus, error) {
	if f.block != nil {
		<-f.block
	}
	f.record("inspect")
	return f.inspect, f.inspectErr
}

func (f *fakeProvisioner) EnsureNetwork(_ context.Context, _ domain.SetupConfig) (domain.NetworkConfig, error) {
	f.record("network")
	if f.ensureErr != nil {
		return domain.NetworkConfig{}, f.ensureErr
	}
	return fakeNetwork(), nil
}

func (f *fakeProvisioner) Network(_ context.Context) (domain.NetworkConfig, error) {
	return fakeNetwork(), nil
}

func fakeNetwork() domain.NetworkConfig {
	return domain.NetworkConfig{
		Interface: "vboxnet1",
		HostIP:    "192.168.57.1",
		Netmask:   "255.255.255.0",
		CIDR:      "192.168.57.0/24",
		DNSIP:     "192.168.57.10",
		DHCPLower: "192.168.57.100",
		DHCPUpper: "192.168.57.199",
	}
}

func (f *fakeProvisioner) CreateVM(_ context.Context, _ domain.SetupConfig, name, _, _ string) (string, error) {
	f.record("create:" + name)
	return "/vms/" + name + ".vdi", f.createErr
}

func (f *fakeProvisioner) CreateSeedISO(_ context.Context, cfg domain.SetupConfig, name, _, _ string) (string, error) {
	f.record("seed:" + name)
	path := filepath.Join(cfg.SeedDir, name+"-seed.iso")
	_ = os.MkdirAll(cfg.SeedDir, 0o755)
	_ = os.WriteFile(path, []byte("iso"), 0o600)
	return path, nil
}

func (f *fakeProvisioner) StartSeedBoot(_ context.Context, _ domain.SetupConfig, name, _ string) error {
	f.record("boot:" + name)
	return nil
}

func (f *fakeProvisioner) WaitGuestReady(_ context.Context, name string, _ time.Duration, _ ...ports.ProgressFunc) error {
	f.record("wait:" + name)
	return f.waitErr
}

func (f *fakeProvisioner) RunGuestScript(_ context.Context, _ domain.SetupConfig, name, script string, _ ...string) error {
	f.record("script:" + name + ":" + filepath.Base(script))
	return f.scriptErr
}

func (f *fakeProvisioner) ReserveDHCPAddress(_ context.Context, _ domain.SetupConfig, name, ip string) error {
	f.record("reserve:" + name + ":" + ip)
	return f.reserveErr
}

func (f *fakeProvisioner) ResetEnvironment(_ context.Context, _ domain.SetupConfig) error {
	f.record("reset")
	return nil
}

func (f *fakeProvisioner) MarkBaseMultiattach(_ context.Context, _ domain.SetupConfig, name string) (string, error) {
	f.record("multiattach:" + name)
	if f.multiErr != nil {
		return "", f.multiErr
	}
	return f.baseDisk, nil
}

type fakeImages struct {
	path      string
	found     bool
	download  string
	err       error
	downloads int
}

func (f *fakeImages) Locate(_ context.Context, _ string) (string, bool) {
	return f.path, f.found
}

func (f *fakeImages) Download(_ context.Context, _, _ string, progress ports.ProgressFunc) (string, error) {
	f.downloads++
	if progress != nil {
		progress(50, "descargando")
	}
	return f.download, f.err
}

func testConfig(dir string) domain.SetupConfig {
	return domain.SetupConfig{
		RawDir:           dir,
		SeedDir:          filepath.Join(dir, "seeds"),
		SeedTemplatesDir: filepath.Join("..", "..", "scripts", "seeds"),
		RawURL:           "https://example.invalid/debian.raw",
		DNSVMName:        "ns",
		BaseVMName:       "ApacheServer",
		DNSHostname:      "ns.cloud.local",
		BaseHostname:     "apache.cloud.local",
		GuestUser:        "http",
		GuestPassword:    "httpaas",
		MemoryMB:         512,
		DiskMB:           8192,
		InstallTimeout:   time.Second,
	}
}

func newService(t *testing.T, prov *fakeProvisioner, images ports.ImageStore) *services.SetupService {
	t.Helper()
	stateFile := filepath.Join(t.TempDir(), "setup-estado.json")
	return services.NewSetupService(testConfig(t.TempDir()), prov, images, nil, stateFile)
}

func waitForIdle(t *testing.T, svc *services.SetupService) domain.SetupState {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		state := svc.Estado()
		if !state.Ejecutando {
			return state
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("la preparación no terminó a tiempo")
	return domain.SetupState{}
}

func configure(t *testing.T, svc *services.SetupService) {
	t.Helper()
	if err := svc.Configurar(domain.SetupInput{Usuario: "http", Contrasena: "httpaas123"}); err != nil {
		t.Fatalf("no se pudo configurar la preparación: %v", err)
	}
}

func TestSetupService_RequiresFormBeforeStarting(t *testing.T) {
	svc := newService(t, &fakeProvisioner{}, &fakeImages{})
	if err := svc.Iniciar(context.Background()); !errors.Is(err, services.ErrSetupSinConfigurar) {
		t.Fatalf("expected ErrSetupSinConfigurar, got %v", err)
	}
}

func TestSetupService_ConfigurarValidatesAndPublishes(t *testing.T) {
	svc := newService(t, &fakeProvisioner{}, &fakeImages{})

	if err := svc.Configurar(domain.SetupInput{Usuario: "A", Contrasena: "x"}); err == nil {
		t.Fatal("expected validation error for an invalid user")
	}
	if err := svc.Configurar(domain.SetupInput{Usuario: "http", Contrasena: " "}); err == nil {
		t.Fatal("expected validation error for an empty password")
	}
	if err := svc.Configurar(domain.SetupInput{Usuario: "http", Contrasena: "httpaas123"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	state := svc.Estado()
	if !state.Configuracion.Lista || state.Configuracion.Usuario != "http" {
		t.Fatalf("unexpected configuration state: %+v", state.Configuracion)
	}
}

func TestSetupService_InitialState(t *testing.T) {
	svc := newService(t, &fakeProvisioner{}, &fakeImages{})

	state := svc.Estado()
	if !state.RequiereConfiguracion {
		t.Fatal("un entorno vacío debe requerir configuración")
	}
	if len(state.Pasos) != 11 {
		t.Fatalf("expected 11 steps, got %d", len(state.Pasos))
	}
	for _, step := range state.Pasos {
		if step.Estado != domain.StepPending {
			t.Errorf("expected step %s pending, got %s", step.ID, step.Estado)
		}
	}
}

func TestSetupService_DetectionReady(t *testing.T) {
	prov := &fakeProvisioner{inspect: domain.EnvironmentStatus{
		VBoxAvailable:       true,
		VBoxVersion:         "7.2.22",
		HasDNSVM:            true,
		HasBaseVM:           true,
		BaseDiskMultiattach: true,
		HostOnlyInterface:   "vboxnet1",
		Ready:               true,
	}}
	svc := newService(t, prov, &fakeImages{found: true, path: "/isos/debian.iso"})
	configure(t, svc)

	if err := svc.ActualizarDeteccion(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	state := svc.Estado()
	if state.Imagen != "/isos/debian.iso" {
		t.Errorf("expected detected image, got %q", state.Imagen)
	}
	if state.RequiereConfiguracion || !state.Completado {
		t.Fatalf("expected a completed environment, got %+v", state)
	}
	for _, step := range state.Pasos {
		if step.Estado != domain.StepCompleted {
			t.Errorf("expected step %s completed, got %s", step.ID, step.Estado)
		}
	}
}

func TestSetupService_RunsFullPipeline(t *testing.T) {
	prov := &fakeProvisioner{
		inspect:  domain.EnvironmentStatus{VBoxAvailable: true, VBoxVersion: "7.2.22", Ready: true},
		baseDisk: "/media/ApacheServer.vdi",
	}
	images := &fakeImages{found: true, path: "/isos/debian-13.iso"}
	svc := newService(t, prov, images)
	configure(t, svc)

	if err := svc.Iniciar(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	state := waitForIdle(t, svc)
	if state.Error != "" {
		t.Fatalf("unexpected error state: %s", state.Error)
	}
	for _, step := range state.Pasos {
		if step.Estado != domain.StepCompleted {
			t.Errorf("expected step %s completed, got %s (%s)", step.ID, step.Estado, step.Mensaje)
		}
	}
	if got := svc.BaseDiskPath(); got != "/media/ApacheServer.vdi" {
		t.Errorf("expected persisted base disk, got %q", got)
	}

	wantCalls := []string{
		"inspect", "network", "create:ns", "seed:ns", "reserve:ns:192.168.57.10", "boot:ns", "wait:ns",
		"script:ns:guest-bind9.sh", "create:ApacheServer", "seed:ApacheServer", "boot:ApacheServer",
		"wait:ApacheServer", "script:ApacheServer:guest-apache.sh", "multiattach:ApacheServer",
	}
	if len(prov.calls) < len(wantCalls) {
		t.Fatalf("expected at least %d calls, got %v", len(wantCalls), prov.calls)
	}
	for i, want := range wantCalls {
		if prov.calls[i] != want {
			t.Fatalf("call %d: expected %q, got %q", i, want, prov.calls[i])
		}
	}
}

func TestSetupService_DownloadsISOWhenMissing(t *testing.T) {
	prov := &fakeProvisioner{inspect: domain.EnvironmentStatus{VBoxAvailable: true, Ready: true}}
	images := &fakeImages{found: false, download: "/isos/debian-descargada.iso"}
	svc := newService(t, prov, images)
	configure(t, svc)

	if err := svc.Iniciar(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	state := waitForIdle(t, svc)
	if state.Error != "" {
		t.Fatalf("unexpected error: %s", state.Error)
	}
	if step, _ := state.Step(domain.SetupStepISO); step.Estado != domain.StepCompleted {
		t.Errorf("expected ISO step completed, got %s", step.Estado)
	}
}

func TestSetupService_RejectsConcurrentRuns(t *testing.T) {
	block := make(chan struct{})
	prov := &fakeProvisioner{block: block, inspect: domain.EnvironmentStatus{VBoxAvailable: true, Ready: true}}
	svc := newService(t, prov, &fakeImages{found: true, path: "/isos/debian.iso"})
	configure(t, svc)

	if err := svc.Iniciar(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := svc.Iniciar(context.Background()); !errors.Is(err, services.ErrSetupEjecutando) {
		t.Fatalf("expected ErrSetupEjecutando, got %v", err)
	}
	close(block)
	waitForIdle(t, svc)
}

func TestSetupService_RejectsRunWhenCompleted(t *testing.T) {
	prov := &fakeProvisioner{inspect: domain.EnvironmentStatus{VBoxAvailable: true, Ready: true}}
	svc := newService(t, prov, &fakeImages{found: true, path: "/isos/debian.iso"})
	configure(t, svc)

	if err := svc.ActualizarDeteccion(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := svc.Iniciar(context.Background()); !errors.Is(err, services.ErrSetupCompletado) {
		t.Fatalf("expected ErrSetupCompletado, got %v", err)
	}
}

func TestSetupService_StepFailureStopsPipeline(t *testing.T) {
	prov := &fakeProvisioner{
		inspect:   domain.EnvironmentStatus{VBoxAvailable: true, Ready: true},
		ensureErr: errors.New("privilegios insuficientes"),
	}
	svc := newService(t, prov, &fakeImages{found: true, path: "/isos/debian.iso"})
	configure(t, svc)

	if err := svc.Iniciar(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	state := waitForIdle(t, svc)

	if state.Error == "" {
		t.Fatal("expected an error in the state")
	}
	networkStep, _ := state.Step(domain.SetupStepNetwork)
	if networkStep.Estado != domain.StepError {
		t.Errorf("expected network step in error, got %s", networkStep.Estado)
	}
	if step, _ := state.Step(domain.SetupStepDNSVM); step.Estado != domain.StepPending {
		t.Errorf("expected pipeline to stop, got DNSVM step %s", step.Estado)
	}
}

func TestSetupService_SubscribersReceiveUpdates(t *testing.T) {
	prov := &fakeProvisioner{inspect: domain.EnvironmentStatus{VBoxAvailable: true, Ready: true}}
	svc := newService(t, prov, &fakeImages{found: true, path: "/isos/debian.iso"})
	configure(t, svc)

	updates, cancel := svc.Subscribe()
	defer cancel()

	if err := svc.Iniciar(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	waitForIdle(t, svc)

	select {
	case state := <-updates:
		if state.ActualizadoEn.IsZero() {
			t.Error("expected an updated snapshot")
		}
	case <-time.After(time.Second):
		t.Fatal("no se recibieron actualizaciones de la preparación")
	}
}

func TestSetupService_ContinuesAfterRequestContextCanceled(t *testing.T) {
	prov := &fakeProvisioner{
		inspect:  domain.EnvironmentStatus{VBoxAvailable: true, Ready: true},
		baseDisk: "/media/ApacheServer.vdi",
	}
	svc := newService(t, prov, &fakeImages{found: true, path: "/isos/debian.iso"})
	configure(t, svc)

	ctx, cancel := context.WithCancel(context.Background())
	if err := svc.Iniciar(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Simula el cierre de la petición HTTP que inició la preparación.
	cancel()

	state := waitForIdle(t, svc)
	if state.Error != "" {
		t.Fatalf("la configuración no debe abortar al cerrar la petición: %s", state.Error)
	}
	if !state.Completado {
		t.Fatalf("expected a completed environment, got %+v", state)
	}
}

func TestSetupService_ReiniciarClearsProgress(t *testing.T) {
	prov := &fakeProvisioner{
		inspect:  domain.EnvironmentStatus{VBoxAvailable: true, Ready: true},
		baseDisk: "/media/ApacheServer.vdi",
	}
	svc := newService(t, prov, &fakeImages{found: true, path: "/isos/debian.raw"})
	configure(t, svc)
	if err := svc.Iniciar(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	waitForIdle(t, svc)

	prov.inspect = domain.EnvironmentStatus{VBoxAvailable: true, Ready: false}
	if err := svc.Reiniciar(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	state := svc.Estado()
	if state.Completado || !state.RequiereConfiguracion {
		t.Fatalf("expected a pending environment, got %+v", state)
	}
	if got := svc.BaseDiskPath(); got != "" {
		t.Errorf("expected the base disk to be cleared, got %q", got)
	}
	found := false
	for _, call := range prov.calls {
		if call == "reset" {
			found = true
		}
	}
	if !found {
		t.Error("expected ResetEnvironment to be called")
	}
}
