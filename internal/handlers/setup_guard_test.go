// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/domain"
	"github.com/Andrey3681/httpaas-orchestrator/internal/handlers"
)

type setupStateStub struct {
	state domain.SetupState
}

func (s setupStateStub) Estado() domain.SetupState { return s.state }

func TestRequireSetupBlocksMutationsUntilCompleted(t *testing.T) {
	guard := handlers.RequireSetup(setupStateStub{state: domain.SetupState{RequiereConfiguracion: true, Completado: false}})
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	handler := guard(next)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/instancias", nil))
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for POST, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/instancias/paisajes", nil))
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for DELETE, got %d", rec.Code)
	}
}

func TestRequireSetupAllowsReadsAndSetupRoutes(t *testing.T) {
	guard := handlers.RequireSetup(setupStateStub{state: domain.SetupState{RequiereConfiguracion: true}})
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	handler := guard(next)

	for _, req := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/api/instancias", nil),
		httptest.NewRequest(http.MethodPost, "/api/setup/iniciar", nil),
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("%s %s: expected 200, got %d", req.Method, req.URL.Path, rec.Code)
		}
	}
}

func TestRequireSetupAllowsMutationsWhenCompleted(t *testing.T) {
	guard := handlers.RequireSetup(setupStateStub{state: domain.SetupState{Completado: true}})
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusCreated) })
	handler := guard(next)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/instancias", nil))
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
}
