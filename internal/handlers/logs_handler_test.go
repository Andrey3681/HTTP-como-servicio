// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package handlers_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Andrey3681/httpaas-orchestrator/internal/handlers"
	"github.com/Andrey3681/httpaas-orchestrator/internal/logging"
)

func TestLogsHandler_List(t *testing.T) {
	t.Parallel()

	broker := logging.NewBroker(10)
	broker.Append(logging.LevelInfo, "test", "hello", map[string]any{"k": "v"})
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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("expected text/event-stream, got %q", resp.Header.Get("Content-Type"))
	}

	broker.Append(logging.LevelInfo, "stream", "live-event", nil)

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
