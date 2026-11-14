// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package handlers_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/domain"
	"github.com/Andrey3681/httpaas-orchestrator/internal/core/ports"
	"github.com/Andrey3681/httpaas-orchestrator/internal/handlers"
	"github.com/Andrey3681/httpaas-orchestrator/internal/services"
)

type stubProvisioner struct {
	ready domain.EnvironmentStatus
}

func (s *stubProvisioner) Inspect(context.Context, domain.SetupConfig) (domain.EnvironmentStatus, error) {
	return s.ready, nil
}

func (s *stubProvisioner) EnsureNetwork(context.Context, domain.SetupConfig) (domain.NetworkConfig, error) {
	return stubNetwork(), nil
}

func (s *stubProvisioner) Network(context.Context) (domain.NetworkConfig, error) {
	return stubNetwork(), nil
}

func stubNetwork() domain.NetworkConfig {
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

func (s *stubProvisioner) ConvertToBaseVDI(context.Context, domain.SetupConfig, string, string, ports.ProgressFunc) error {
	return nil
}

func (s *stubProvisioner) CreateVM(context.Context, domain.SetupConfig, string, string, string) (string, error) {
	return "/vms/disk.vdi", nil
}

func (s *stubProvisioner) CreateSeedISO(context.Context, domain.SetupConfig, string, string, string) (string, error) {
	return "/seeds/seed.iso", nil
}

func (s *stubProvisioner) StartSeedBoot(context.Context, domain.SetupConfig, string, string) error {
	return nil
}

func (s *stubProvisioner) WaitGuestReady(context.Context, string, time.Duration, ...ports.ProgressFunc) error {
	return nil
}

func (s *stubProvisioner) RunGuestScript(context.Context, domain.SetupConfig, string, string, ...string) (string, error) {
	return "", nil
}

func (s *stubProvisioner) ReserveDHCPAddress(context.Context, domain.SetupConfig, string, string) error {
	return nil
}

func (s *stubProvisioner) ResetEnvironment(context.Context, domain.SetupConfig) error {
	return nil
}

func (s *stubProvisioner) MarkBaseMultiattach(context.Context, domain.SetupConfig, string) (string, error) {
	return "/media/ApacheServer.vdi", nil
}

type stubImages struct{ path string }

func (s *stubImages) Locate(context.Context, string) (string, bool) { return s.path, s.path != "" }
func (s *stubImages) Download(_ context.Context, _, _ string, progress ports.ProgressFunc) (string, error) {
	if progress != nil {
		progress(100, "ok")
	}
	return s.path, nil
}

func newSetupHandler(t *testing.T, ready bool) (*handlers.SetupHandler, *services.SetupService) {
	t.Helper()
	cfg := domain.SetupConfig{
		RawDir:           t.TempDir(),
		SeedDir:          filepath.Join(t.TempDir(), "seeds"),
		SeedTemplatesDir: filepath.Join("..", "..", "scripts", "seeds"),
		DNSVMName:        "ns",
		BaseVMName:       "ApacheServer",
		GuestUser:        "http",
	}
	prov := &stubProvisioner{ready: domain.EnvironmentStatus{
		VBoxAvailable:       true,
		HasDNSVM:            ready,
		HasBaseVM:           ready,
		BaseDiskMultiattach: ready,
		Ready:               ready,
	}}
	stateFile := filepath.Join(t.TempDir(), "setup-estado.json")
	svc := services.NewSetupService(cfg, prov, &stubImages{path: "/isos/debian.iso"}, nil, stateFile)
	return handlers.NewSetupHandler(svc), svc
}

func TestSetupHandler_Estado(t *testing.T) {
	handler, _ := newSetupHandler(t, false)
	req := httptest.NewRequest(http.MethodGet, "/api/setup/estado", nil)
	rec := httptest.NewRecorder()

	handler.HandleEstado(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var state domain.SetupState
	if err := json.NewDecoder(rec.Body).Decode(&state); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !state.RequiereConfiguracion {
		t.Error("expected configuration to be required")
	}
	if len(state.Pasos) == 0 {
		t.Error("expected steps in the response")
	}
}

func TestSetupHandler_MethodNotAllowed(t *testing.T) {
	handler, _ := newSetupHandler(t, false)

	rec := httptest.NewRecorder()
	handler.HandleEstado(rec, httptest.NewRequest(http.MethodPost, "/api/setup/estado", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for estado, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	handler.HandleIniciar(rec, httptest.NewRequest(http.MethodGet, "/api/setup/iniciar", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for iniciar, got %d", rec.Code)
	}
}

func postSetupConfig(t *testing.T, handler *handlers.SetupHandler, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/setup/configuracion", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	handler.HandleConfigurar(rec, req)
	return rec
}

func TestSetupHandler_ConfigurarValidation(t *testing.T) {
	handler, _ := newSetupHandler(t, false)

	rec := postSetupConfig(t, handler, `{"usuario":"http","contrasena":"httpaas123"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}

	rec = postSetupConfig(t, handler, `{"usuario":"A","contrasena":"x"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid credentials, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	handler.HandleConfigurar(rec, httptest.NewRequest(http.MethodGet, "/api/setup/configuracion", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestSetupHandler_IniciarWithoutConfiguration(t *testing.T) {
	handler, _ := newSetupHandler(t, true)
	rec := httptest.NewRecorder()
	handler.HandleIniciar(rec, httptest.NewRequest(http.MethodPost, "/api/setup/iniciar", nil))
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}

func TestSetupHandler_IniciarAccepted(t *testing.T) {
	handler, svc := newSetupHandler(t, true)
	if rec := postSetupConfig(t, handler, `{"usuario":"http","contrasena":"httpaas123"}`); rec.Code != http.StatusOK {
		t.Fatalf("unexpected configuration status: %d", rec.Code)
	}
	rec := httptest.NewRecorder()

	handler.HandleIniciar(rec, httptest.NewRequest(http.MethodPost, "/api/setup/iniciar", nil))

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d (%s)", rec.Code, rec.Body.String())
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !svc.Estado().Ejecutando {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Error("la configuración no terminó antes del cierre de la prueba")
}

func TestSetupHandler_IniciarConflictWhenCompleted(t *testing.T) {
	handler, svc := newSetupHandler(t, true)
	if rec := postSetupConfig(t, handler, `{"usuario":"http","contrasena":"httpaas123"}`); rec.Code != http.StatusOK {
		t.Fatalf("unexpected configuration status: %d", rec.Code)
	}
	if err := svc.ActualizarDeteccion(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rec := httptest.NewRecorder()
	handler.HandleIniciar(rec, httptest.NewRequest(http.MethodPost, "/api/setup/iniciar", nil))

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}

func TestSetupHandler_StreamSendsSnapshot(t *testing.T) {
	handler, _ := newSetupHandler(t, false)
	server := httptest.NewServer(http.HandlerFunc(handler.HandleStream))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("unexpected content type: %s", resp.Header.Get("Content-Type"))
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var state domain.SetupState
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &state); err != nil {
			t.Fatalf("invalid SSE payload: %v", err)
		}
		if len(state.Pasos) == 0 {
			t.Error("expected steps in the streamed snapshot")
		}
		return
	}
	t.Fatal("no SSE data received")
}

func TestSetupHandler_Reiniciar(t *testing.T) {
	handler, _ := newSetupHandler(t, false)
	rec := httptest.NewRecorder()
	handler.HandleReiniciar(rec, httptest.NewRequest(http.MethodPost, "/api/setup/reiniciar", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.HandleReiniciar(rec, httptest.NewRequest(http.MethodGet, "/api/setup/reiniciar", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}
