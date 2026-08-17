package tracing

import (
	"sync"

	"github.com/xhanio/errors"
)

type registry struct {
	mu        sync.RWMutex
	factories map[string]Factory
}

var reg = &registry{factories: map[string]Factory{}}

func (r *registry) register(name string, f Factory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.factories[name]; exists {
		panic("tracing: exporter already registered: " + name)
	}
	r.factories[name] = f
}

func (r *registry) lookup(name string) (Factory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.factories[name]
	if !ok {
		return nil, errors.Newf("unsupported tracing exporter: %s (not registered — blank-import the corresponding pkg/services/tracing/exporters/* package)", name)
	}
	return f, nil
}

func lookupFactory(name string) (Factory, error) {
	return reg.lookup(name)
}
