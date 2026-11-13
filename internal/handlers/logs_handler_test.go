// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package handlers_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Andrey3681/httpaas-orchestrator/internal/handlers"
	"github.com/Andrey3681/httpaas-orchestrator/internal/logging"
)

func strconvI64(v int64) string { return strconv.FormatInt(v, 10) }

func TestLogsHandler_List(t *testing.T) {
	t.Parallel()

	broker := logging.NewBroker(10)
	broker.Append(string(logging.LevelInfo), "test", "hello", map[string]any{"k": "v"})
	handler := handlers.NewLogsHandler(broker)

	req := httptest.NewRequest(http.MethodGet, "/api/logs?limit=5", nil)
	rec := httptest.NewRecorder()

	handler.HandleLogs(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	datos, ok := body["datos"].([]any)
	if !ok || len(datos) != 1 {
		t.Fatalf("expected 1 log entry, got %#v", body["datos"])
	}
	entry, ok := datos[0].(map[string]any)
	if !ok || entry["mensaje"] != "hello" || entry["componente"] != "test" {
		t.Fatalf("unexpected log entry: %#v", datos[0])
	}
}

func TestLogsHandler_ListInvalidLimit(t *testing.T) {
	t.Parallel()

	handler := handlers.NewLogsHandler(logging.NewBroker(10))
	req := httptest.NewRequest(http.MethodGet, "/api/logs?limit=abc", nil)
	rec := httptest.NewRecorder()

	handler.HandleLogs(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLogsHandler_ListFilters(t *testing.T) {
	t.Parallel()

	broker := logging.NewBroker(10)
	broker.Append(string(logging.LevelInfo), "orchestrator", "a", nil)
	target := broker.AppendEntry(string(logging.LevelError), "bind9", "b", nil)
	broker.Append(string(logging.LevelInfo), "bind9", "c", nil)
	handler := handlers.NewLogsHandler(broker)

	req := httptest.NewRequest(http.MethodGet, "/api/logs?nivel=ERROR", nil)
	rec := httptest.NewRecorder()
	handler.HandleLogs(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if datos, _ := body["datos"].([]any); len(datos) != 1 {
		t.Fatalf("expected 1 filtered entry, got %#v", body["datos"])
	}

	req = httptest.NewRequest(http.MethodGet, "/api/logs?since="+strconvI64(target.ID)+"&componente=bind9", nil)
	rec = httptest.NewRecorder()
	handler.HandleLogs(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if datos, _ := body["datos"].([]any); len(datos) != 1 {
		t.Fatalf("expected 1 since+component entry, got %#v", body["datos"])
	}
}

func TestLogsHandler_Clear(t *testing.T) {
	t.Parallel()

	broker := logging.NewBroker(10)
	broker.Append(string(logging.LevelInfo), "test", "x", nil)
	handler := handlers.NewLogsHandler(broker)

	req := httptest.NewRequest(http.MethodDelete, "/api/logs", nil)
	rec := httptest.NewRecorder()
	handler.HandleLogs(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := broker.Recent(10); len(got) != 0 {
		t.Fatalf("expected broker cleared, got %d entries", len(got))
	}
}

func TestLogsHandler_ListMethodNotAllowed(t *testing.T) {
	t.Parallel()

	handler := handlers.NewLogsHandler(logging.NewBroker(10))
	req := httptest.NewRequest(http.MethodPost, "/api/logs", nil)
	rec := httptest.NewRecorder()

	handler.HandleLogs(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLogsHandler_Stream(t *testing.T) {
	t.Parallel()

	broker := logging.NewBroker(10)
	handler := handlers.NewLogsHandler(broker)
	server := httptest.NewServer(http.HandlerFunc(handler.HandleStream))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
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

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("expected text/event-stream, got %q", resp.Header.Get("Content-Type"))
	}

	broker.Append(string(logging.LevelInfo), "stream", "live-event", nil)

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			var entry map[string]any
			if err := json.Unmarshal([]byte(payload), &entry); err != nil {
				t.Fatalf("invalid SSE data JSON: %v (%q)", err, payload)
			}
			if entry["mensaje"] == "live-event" {
				return
			}
		}
	}
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		t.Fatalf("read stream: %v", err)
	}
	t.Fatal("did not receive expected SSE event")
}

func TestLogsHandler_StreamResume(t *testing.T) {
	t.Parallel()

	broker := logging.NewBroker(10)
	first := broker.AppendEntry(string(logging.LevelInfo), "test", "missed", nil)
	broker.Append(string(logging.LevelInfo), "test", "replay", nil)
	handler := handlers.NewLogsHandler(broker)
	server := httptest.NewServer(http.HandlerFunc(handler.HandleStream))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"?since="+strconv.FormatInt(first.ID, 10), nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			var entry map[string]any
			if err := json.Unmarshal([]byte(payload), &entry); err != nil {
				t.Fatalf("invalid SSE data JSON: %v", err)
			}
			if entry["mensaje"] == "replay" {
				return
			}
		}
	}
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		t.Fatalf("read stream: %v", err)
	}
	t.Fatal("did not receive resumed SSE event")
}
