package messagebus

import (
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/nameutil"
)

type Option func(*manager)

func (m *manager) applyOptions(opts ...Option) {
	for _, opt := range opts {
		opt(m)
	}
}

func (m *manager) setDefaults() {
	if m.log == nil {
		m.log = log.Default
	}
	if m.name == "" {
		m.name = nameutil.Name(m)
	}
	if m.topic == "" {
		m.topic = DefaultTopic
	}
}

// Construction options: fixed for the instance's life, set at New (design/config.md).

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

// WithTopic lets several messagebus instances share one pubsub.
func WithTopic(topic string) Option {
	return func(m *manager) {
		m.topic = topic
	}
}
