// Package otelutil holds the shared plumbing behind framingo's telemetry
// services (tracing, metrics): the name-to-factory Registry their exporter
// subpackages register into from init(), and the transport-agnostic otlp
// config vocabulary (Duration, Retry, Config, the strict Decode) their
// factories build exporters from. Signal-specific knobs layer on top in the
// signal's own package - tracing's exporters/model wraps Config with the
// span-exporter-only max_request_size, which is why it is absent here. It
// also holds the HTTP convention helpers the api server's metrics and the
// trace middleware share (RequestMethod), so a span and a metric never
// disagree about the same request.
package otelutil

import (
	"sync"

	"github.com/xhanio/errors"
)

// Registry is the name-to-factory registry behind pluggable implementations
// that register themselves from subpackage init()s - the discipline the
// tracing and metrics exporter registries share: Register panics on a
// duplicate name so misconfiguration surfaces at startup, and a failed
// Lookup names the blank-import that would have provided it.
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

// Register makes f available under name. Double-registration panics so a
// conflicting init() surfaces at startup instead of silently winning.
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
