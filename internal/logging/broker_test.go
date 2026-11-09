// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package logging_test

import (
	"sync"
	"testing"
	"time"

	"github.com/Andrey3681/httpaas-orchestrator/internal/logging"
)

func TestBroker_AppendAndRecent(t *testing.T) {
	t.Parallel()

	broker := logging.NewBroker(3)
	broker.Append(logging.LevelInfo, "test", "first", nil)
	second := broker.Append(logging.LevelWarn, "test", "second", map[string]any{"k": "v"})
	broker.Append(logging.LevelError, "test", "third", nil)
	broker.Append(logging.LevelInfo, "test", "fourth", nil)

	entries := broker.Recent(10)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Mensaje != "second" || entries[2].Mensaje != "fourth" {
		t.Fatalf("unexpected ring buffer contents: %#v", entries)
	}
	if second.ID == 0 || entries[2].ID <= second.ID {
		t.Fatalf("expected monotonic IDs, got second=%d last=%d", second.ID, entries[2].ID)
	}
	if entries[0].Nivel != logging.LevelWarn || entries[0].Campos["k"] != "v" {
		t.Fatalf("unexpected entry payload: %#v", entries[0])
	}
	if entries[0].TS.IsZero() || entries[0].TS.After(time.Now().UTC().Add(time.Second)) {
		t.Fatalf("unexpected timestamp: %v", entries[0].TS)
	}
}

func TestBroker_RecentLimit(t *testing.T) {
	t.Parallel()

	broker := logging.NewBroker(10)
	for i := 0; i < 5; i++ {
		broker.Append(logging.LevelInfo, "test", "msg", nil)
	}

	entries := broker.Recent(2)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}

func TestBroker_Subscribe(t *testing.T) {
	t.Parallel()

	broker := logging.NewBroker(10)
	ch, cancel := broker.Subscribe()
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case entry := <-ch:
			if entry.Mensaje != "live" || entry.Componente != "stream" {
				t.Errorf("unexpected entry: %#v", entry)
			}
		case <-time.After(2 * time.Second):
			t.Error("timed out waiting for subscribed entry")
		}
	}()

	broker.Append(logging.LevelInfo, "stream", "live", nil)
	wg.Wait()
}
