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
	broker.AppendEntry(string(logging.LevelInfo), "test", "first", nil)
	second := broker.AppendEntry(string(logging.LevelWarn), "test", "second", map[string]any{"k": "v"})
	broker.AppendEntry(string(logging.LevelError), "test", "third", nil)
	broker.AppendEntry(string(logging.LevelInfo), "test", "fourth", nil)

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
		broker.Append(string(logging.LevelInfo), "test", "msg", nil)
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

	broker.Append(string(logging.LevelInfo), "stream", "live", nil)
	wg.Wait()
}

func TestBroker_Clear(t *testing.T) {
	t.Parallel()

	broker := logging.NewBroker(10)
	broker.AppendEntry(string(logging.LevelInfo), "test", "one", nil)
	broker.Clear()
	if got := broker.Recent(10); len(got) != 0 {
		t.Fatalf("expected empty ring after clear, got %d", len(got))
	}
	broker.AppendEntry(string(logging.LevelInfo), "test", "two", nil)
	if got := broker.Recent(10); len(got) != 1 || got[0].Mensaje != "two" {
		t.Fatalf("unexpected entries after clear+append: %#v", got)
	}
}

func TestBroker_QueryFilters(t *testing.T) {
	t.Parallel()

	broker := logging.NewBroker(10)
	broker.AppendEntry(string(logging.LevelInfo), "orchestrator", "a", nil)
	target := broker.AppendEntry(string(logging.LevelError), "bind9", "b", nil)
	broker.AppendEntry(string(logging.LevelInfo), "bind9", "c", nil)

	byLevel := broker.Query(logging.QueryOptions{Level: string(logging.LevelError)})
	if len(byLevel) != 1 || byLevel[0].Mensaje != "b" {
		t.Fatalf("unexpected level filter result: %#v", byLevel)
	}
	byComponent := broker.Query(logging.QueryOptions{Component: "bind9"})
	if len(byComponent) != 2 {
		t.Fatalf("expected 2 bind9 entries, got %#v", byComponent)
	}
	since := broker.Query(logging.QueryOptions{Since: target.ID})
	if len(since) != 1 || since[0].Mensaje != "c" {
		t.Fatalf("unexpected since filter result: %#v", since)
	}
}
