// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/domain"
	"github.com/Andrey3681/httpaas-orchestrator/internal/handlers"
)

type stubDnsProvider struct {
	records []domain.DnsRecord
}

func (s *stubDnsProvider) AddRecord(_ context.Context, _ domain.DnsRecord) error { return nil }

func (s *stubDnsProvider) RemoveRecord(_ context.Context, _ string) error { return nil }

func (s *stubDnsProvider) Resolve(_ context.Context, _ string) (netip.Addr, error) {
	return netip.MustParseAddr("192.168.10.30"), nil
}

func (s *stubDnsProvider) ListRecords(_ context.Context) ([]domain.DnsRecord, error) {
	return s.records, nil
}

func TestDnsHandler_Health(t *testing.T) {
	t.Parallel()

	handler := handlers.NewDnsHandler(&stubDnsProvider{}, "cloud.local", nil)
	req := httptest.NewRequest(http.MethodGet, "/api/salud", nil)
	rec := httptest.NewRecorder()

	handler.HandleHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if _, ok := body["estado"]; !ok {
		t.Fatalf("expected 'estado' field in health response, got %#v", body)
	}
}

func TestDnsHandler_Records(t *testing.T) {
	t.Parallel()

	handler := handlers.NewDnsHandler(&stubDnsProvider{}, "cloud.local", nil)
	req := httptest.NewRequest(http.MethodGet, "/api/dns/registros", nil)
	rec := httptest.NewRecorder()

	handler.HandleRecords(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if _, ok := body["zona"]; !ok {
		t.Fatalf("expected 'zona' field in records response, got %#v", body)
	}
	registros, ok := body["registros"]
	if !ok {
		t.Fatalf("expected 'registros' field in records response, got %#v", body)
	}
	if _, ok := registros.([]any); !ok {
		t.Fatalf("expected 'registros' to be an array, got %T (%#v)", registros, registros)
	}
}

func TestDnsHandler_RecordsMethodNotAllowed(t *testing.T) {
	t.Parallel()

	handler := handlers.NewDnsHandler(&stubDnsProvider{}, "cloud.local", nil)
	req := httptest.NewRequest(http.MethodPost, "/api/dns/registros", nil)
	rec := httptest.NewRecorder()

	handler.HandleRecords(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDnsHandler_RecordsFromProvider(t *testing.T) {
	t.Parallel()

	handler := handlers.NewDnsHandler(&stubDnsProvider{
		records: []domain.DnsRecord{{
			Name: "www.cloud.local.",
			Zone: "cloud.local.",
			Type: "A",
			IP:   "192.168.10.30",
			TTL:  300,
		}},
	}, "cloud.local", nil)
	req := httptest.NewRequest(http.MethodGet, "/api/dns/registros", nil)
	rec := httptest.NewRecorder()

	handler.HandleRecords(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	registros, ok := body["registros"].([]any)
	if !ok || len(registros) != 1 {
		t.Fatalf("expected 1 registro, got %#v", body["registros"])
	}
	item, ok := registros[0].(map[string]any)
	if !ok || item["ip"] != "192.168.10.30" {
		t.Fatalf("unexpected registro payload: %#v", registros[0])
	}
}
