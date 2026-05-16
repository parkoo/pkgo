// Package tag provides a concurrency-safe key-value tag carrier via context.
package tag

import (
	"context"
	"maps"
	"sync"
)

type ctxMarker struct{}

var ctxMarkerKey = &ctxMarker{}

// Tags is a concurrency-safe key-value collection carried through context.
type Tags struct {
	mux   sync.RWMutex
	value map[string]any
}

// NewTags creates a new empty Tags instance.
func NewTags() *Tags {
	return &Tags{value: make(map[string]any)}
}

// Set stores a key-value pair. It is safe for concurrent use.
func (t *Tags) Set(k string, v any) {
	t.mux.Lock()
	defer t.mux.Unlock()
	t.value[k] = v
}

// Get returns the value for the given key and whether it exists.
func (t *Tags) Get(k string) (any, bool) {
	t.mux.RLock()
	defer t.mux.RUnlock()
	v, ok := t.value[k]
	return v, ok
}

// Values returns a shallow copy of all key-value pairs.
// The returned map is safe to read and write without affecting the original data.
func (t *Tags) Values() map[string]any {
	t.mux.RLock()
	defer t.mux.RUnlock()
	cp := make(map[string]any, len(t.value))
	maps.Copy(cp, t.value)
	return cp
}

// Has reports whether the given key exists.
func (t *Tags) Has(k string) bool {
	t.mux.RLock()
	defer t.mux.RUnlock()
	_, ok := t.value[k]
	return ok
}

// SetInContext stores the Tags into the given context.
func SetInContext(ctx context.Context, tags *Tags) context.Context {
	return context.WithValue(ctx, ctxMarkerKey, tags)
}

// Extract retrieves Tags from ctx. If ctx is nil, context.Background() is used.
// If no Tags exist in ctx, a new one is created and injected automatically.
// Callers should always use the returned ctx, as it may differ from the input.
func Extract(ctx context.Context) (context.Context, *Tags) {
	if ctx == nil {
		ctx = context.Background()
	}
	t, ok := ctx.Value(ctxMarkerKey).(*Tags)
	if !ok {
		t = &Tags{value: make(map[string]any)}
		ctx = SetInContext(ctx, t)
	}
	return ctx, t
}

// Has reports whether ctx contains Tags. Returns false if ctx is nil.
func Has(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	_, ok := ctx.Value(ctxMarkerKey).(*Tags)
	return ok
}
