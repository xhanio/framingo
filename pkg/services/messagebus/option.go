package messagebus

import (
	"github.com/xhanio/framingo/pkg/utils/log"
)

type Option func(*manager)

func (m *manager) apply(opts ...Option) {
	for _, opt := range opts {
		opt(m)
	}
}

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

// WithTopic overrides the default bus topic ("/messages"). Use this when you
// need multiple messagebus instances sharing one pubsub.
func WithTopic(topic string) Option {
	return func(m *manager) {
		m.topic = topic
	}
}
