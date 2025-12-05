package pubsub

import (
	"github.com/xhanio/framingo/pkg/utils/log"
)

type Option func(*manager)

func WithLogger(logger log.Logger) Option {
	return func(m *manager) {
		m.log = logger
	}
}

func WithName(name string) Option {
	return func(m *manager) {
		m.name = name
	}
}

// WithBackend sets a custom backend for the pub/sub manager.
// By default, the manager uses an in-memory backend.
func WithBackend(backend Backend) Option {
	return func(m *manager) {
		m.backend = backend
	}
}
