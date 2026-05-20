package otp

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// memoryEntry holds a value with its expiration time.
type memoryEntry struct {
	value  string
	expAt  time.Time
	intVal int64 // used for counter operations
}

// isExpired checks if the entry has expired.
func (e *memoryEntry) isExpired() bool {
	if e.expAt.IsZero() {
		return false
	}
	return time.Now().After(e.expAt)
}

// MemoryStore is an in-memory Store implementation for development and testing.
// NOT suitable for production (no persistence, no multi-instance sharing).
type MemoryStore struct {
	mu   sync.RWMutex
	data map[string]*memoryEntry
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		data: make(map[string]*memoryEntry),
	}
}

func (m *MemoryStore) Set(_ context.Context, key, value string, exp time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var expAt time.Time
	if exp > 0 {
		expAt = time.Now().Add(exp)
	}
	m.data[key] = &memoryEntry{value: value, expAt: expAt}
	return nil
}

func (m *MemoryStore) Get(_ context.Context, key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, ok := m.data[key]
	if !ok || entry.isExpired() {
		return "", nil
	}
	return entry.value, nil
}

func (m *MemoryStore) Del(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.data, key)
	return nil
}

func (m *MemoryStore) Exists(_ context.Context, key string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, ok := m.data[key]
	if !ok || entry.isExpired() {
		return false, nil
	}
	return true, nil
}

func (m *MemoryStore) Incr(_ context.Context, key string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.data[key]
	if !ok || entry.isExpired() {
		m.data[key] = &memoryEntry{value: "1", intVal: 1}
		return 1, nil
	}
	entry.intVal++
	entry.value = fmt.Sprintf("%d", entry.intVal)
	return entry.intVal, nil
}

func (m *MemoryStore) Expire(_ context.Context, key string, exp time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.data[key]
	if !ok {
		return nil
	}
	entry.expAt = time.Now().Add(exp)
	return nil
}


