// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package logging

import (
	"strings"
	"sync"
	"time"
)

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
func (b *Broker) Append(level Level, component, message string, fields map[string]any) Entry {
	b.mu.Lock()
	b.nextID++
	entry := Entry{
		ID:         b.nextID,
		TS:         time.Now().UTC(),
		Nivel:      level,
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

// Recent retorna los eventos más recientes, ordenados del más antiguo al más nuevo, limitados por limit.
func (b *Broker) Recent(limit int) []Entry {
	b.mu.Lock()
	defer b.mu.Unlock()

	if limit <= 0 || limit > b.size {
		limit = b.size
	}
	result := make([]Entry, 0, limit)
	start := b.size - limit
	for i := start; i < b.size; i++ {
		idx := (b.next - b.size + i + b.capacity) % b.capacity
		result = append(result, b.entries[idx])
	}
	return result
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
		b.Append(LevelInfo, "system", message, nil)
	}
	return len(p), nil
}
