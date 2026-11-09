// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package logging

import (
	"strings"
	"sync"
	"time"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/ports"
)

var _ ports.EventLogger = (*Broker)(nil)

// Broker almacena eventos recientes en un anillo y los distribuye a suscriptores en tiempo real.
type Broker struct {
	mu          sync.Mutex
	capacity    int
	entries     []Entry
	next        int
	size        int
	nextID      int64
	subscribers map[int]chan Entry
	nextSub     int
}

// NewBroker crea un broker con capacidad fija para el anillo de eventos.
func NewBroker(capacity int) *Broker {
	if capacity <= 0 {
		capacity = 1000
	}
	return &Broker{
		capacity:    capacity,
		entries:     make([]Entry, capacity),
		subscribers: make(map[int]chan Entry),
	}
}

// Append registra un evento, lo guarda en el anillo y lo notifica a los suscriptores.
// Implementa la interfaz ports.EventLogger usando level como cadena.
func (b *Broker) Append(level string, component, message string, fields map[string]any) {
	b.AppendEntry(level, component, message, fields)
}

// AppendEntry registra un evento y retorna la entrada creada.
func (b *Broker) AppendEntry(level string, component, message string, fields map[string]any) Entry {
	b.mu.Lock()
	b.nextID++
	entry := Entry{
		ID:         b.nextID,
		TS:         time.Now().UTC(),
		Nivel:      Level(level),
		Componente: component,
		Mensaje:    message,
	}
	if len(fields) > 0 {
		entry.Campos = fields
	}

	b.entries[b.next] = entry
	b.next = (b.next + 1) % b.capacity
	if b.size < b.capacity {
		b.size++
	}

	for _, ch := range b.subscribers {
		select {
		case ch <- entry:
		default:
			// Se descarta para evitar bloquear productores ante consumidores lentos.
		}
	}
	b.mu.Unlock()

	return entry
}

// Logf registra un evento con nivel tipado.
func (b *Broker) Logf(level Level, component, message string, fields map[string]any) Entry {
	return b.AppendEntry(string(level), component, message, fields)
}

// QueryOptions define filtros para consultar eventos del anillo.
type QueryOptions struct {
	Limit     int
	Level     string
	Component string
	Since     int64
}

// Query retorna eventos que cumplen los filtros, ordenados del más antiguo al más nuevo.
func (b *Broker) Query(opts QueryOptions) []Entry {
	b.mu.Lock()
	defer b.mu.Unlock()

	if opts.Limit <= 0 || opts.Limit > b.size {
		opts.Limit = b.size
	}

	all := make([]Entry, 0, b.size)
	for i := 0; i < b.size; i++ {
		idx := (b.next - b.size + i + b.capacity) % b.capacity
		entry := b.entries[idx]
		if opts.Since > 0 && entry.ID <= opts.Since {
			continue
		}
		if opts.Level != "" && string(entry.Nivel) != opts.Level {
			continue
		}
		if opts.Component != "" && entry.Componente != opts.Component {
			continue
		}
		all = append(all, entry)
	}
	if len(all) > opts.Limit {
		all = all[len(all)-opts.Limit:]
	}
	return all
}

// Recent retorna los eventos más recientes, ordenados del más antiguo al más nuevo, limitados por limit.
func (b *Broker) Recent(limit int) []Entry {
	return b.Query(QueryOptions{Limit: limit})
}

// Since retorna eventos con id mayor al cursor indicado.
func (b *Broker) Since(id int64) []Entry {
	return b.Query(QueryOptions{Since: id})
}

// Clear elimina todos los eventos del anillo sin afectar suscriptores.
func (b *Broker) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries = make([]Entry, b.capacity)
	b.next = 0
	b.size = 0
}

// Subscribe registra un suscriptor para eventos futuros y retorna un canal y una función de cancelación.
func (b *Broker) Subscribe() (<-chan Entry, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := b.nextSub
	b.nextSub++
	ch := make(chan Entry, 64)
	b.subscribers[id] = ch

	cancel := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if existing, ok := b.subscribers[id]; ok {
			delete(b.subscribers, id)
			close(existing)
		}
	}
	return ch, cancel
}

// Write implementa io.Writer como puente para el paquete log estándar.
func (b *Broker) Write(p []byte) (int, error) {
	message := strings.TrimSpace(string(p))
	if message != "" {
		b.Append(string(LevelInfo), "system", message, nil)
	}
	return len(p), nil
}
