// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package handlers_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/domain"
	"github.com/Andrey3681/httpaas-orchestrator/internal/handlers"
	"github.com/Andrey3681/httpaas-orchestrator/internal/services"
)

type actionOperator struct{ calls int }

func (o *actionOperator) RestartWebService(context.Context, string, netip.Addr) error {
	o.calls++
	return nil
}

type actionDeployer struct{ calls int }

func (d *actionDeployer) DeployWebZip(context.Context, string, netip.Addr, io.Reader, int64) error {
	d.calls++
	return nil
}

func actionHandler(status domain.InstanceStatus) (*handlers.InstanceHandler, *actionOperator, *actionDeployer, *inMemoryRepo) {
	repo := &inMemoryRepo{instances: map[string]domain.Instance{
		"mario": {
			ID:        "mario",
			Name:      "mario",
			FQDN:      "mario.cloud.local",
			IP:        "192.168.57.105/24",
			Status:    status,
			CreatedAt: time.Now().UTC(),
		},
	}}
	op := &actionOperator{}
	dep := &actionDeployer{}
	svc := services.NewInstanceService(repo, &dummyHypervisor{}, &dummyDnsProvider{}, nil, services.InstanceOptions{
		Operator:   op,
		Deployer:   dep,
		GuestUser:  func() string { return "jorge" },
		SSHKeyPath: "/keys/id_ed25519",
	})
	return handlers.NewInstanceHandler(nil, svc, nil), op, dep, repo
}

func TestInstanceHandler_Access(t *testing.T) {
	handler, _, _, _ := actionHandler(domain.StatusRunning)

	req := httptest.NewRequest(http.MethodGet, "/api/instancias/mario/acceso", nil)
	rec := httptest.NewRecorder()
	handler.HandleInstance(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Datos struct {
			Usuario string `json:"usuario"`
			Comando string `json:"comando"`
			Host    string `json:"host"`
		} `json:"datos"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("respuesta inválida: %v", err)
	}
	if body.Datos.Usuario != "jorge" || body.Datos.Host != "192.168.57.105" || body.Datos.Comando == "" {
		t.Errorf("datos de acceso incompletos: %+v", body.Datos)
	}
}

func TestInstanceHandler_RestartServices(t *testing.T) {
	handler, op, _, _ := actionHandler(domain.StatusRunning)

	req := httptest.NewRequest(http.MethodPost, "/api/instancias/mario/servicios/reiniciar", nil)
	rec := httptest.NewRecorder()
	handler.HandleInstance(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if op.calls != 1 {
		t.Errorf("expected one restart, got %d", op.calls)
	}
}

func TestInstanceHandler_RestartServicesConflict(t *testing.T) {
	handler, _, _, _ := actionHandler(domain.StatusError)

	req := httptest.NewRequest(http.MethodPost, "/api/instancias/mario/servicios/reiniciar", nil)
	rec := httptest.NewRecorder()
	handler.HandleInstance(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rec.Code)
	}
}

func TestInstanceHandler_Redeploy(t *testing.T) {
	handler, _, dep, _ := actionHandler(domain.StatusRunning)

	var zipBody bytes.Buffer
	zw := zip.NewWriter(&zipBody)
	file, _ := zw.Create("index.html")
	_, _ = file.Write([]byte("<h1>nuevo</h1>"))
	_ = zw.Close()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, _ := mw.CreateFormFile("archivo", "sitio.zip")
	_, _ = part.Write(zipBody.Bytes())
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/instancias/mario/despliegue", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	handler.HandleInstance(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if dep.calls != 1 {
		t.Errorf("expected one deploy, got %d", dep.calls)
	}
}

func TestInstanceHandler_UnknownAction(t *testing.T) {
	handler, _, _, _ := actionHandler(domain.StatusRunning)

	req := httptest.NewRequest(http.MethodGet, "/api/instancias/mario/otra-cosa", nil)
	rec := httptest.NewRecorder()
	handler.HandleInstance(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}
