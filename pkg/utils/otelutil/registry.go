// Package otelutil is plumbing shared by the tracing and metrics services: exporter registry, otlp config, HTTP semconv helpers.
package otelutil

import (
	"sync"

	"github.com/xhanio/errors"
)

// Registry is safe for concurrent use; a failed Lookup names the blank-import that would provide it.
type Registry[F any] struct {
	kind string // what a factory builds, e.g. "metrics exporter" - message vocabulary
	hint string // the blank-import guidance, e.g. "pkg/services/metrics/exporters/*"

	mu        sync.RWMutex
	factories map[string]F
}

func NewRegistry[F any](kind, hint string) *Registry[F] {
	return &Registry[F]{
		kind:      kind,
		hint:      hint,
		factories: map[string]F{},
	}
}

// Register panics on a duplicate name, so a conflicting init() fails at startup.
func (r *Registry[F]) Register(name string, f F) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.factories[name]; exists {
		panic(r.kind + " already registered: " + name)
	}
	r.factories[name] = f
}

func (r *Registry[F]) Lookup(name string) (F, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.factories[name]
	if !ok {
		var zero F
		return zero, errors.Newf("unsupported %s: %s (not registered — blank-import the corresponding %s package)", r.kind, name, r.hint)
	}
	return f, nil
}
