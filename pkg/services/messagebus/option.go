package messagebus

import (
	"time"

	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/nameutil"
)

// Default ping settings used by AttachWebSocket when WithPing is not provided.
// Setting WithPing(0, ...) disables server-side pings.
const (
	DefaultPingInterval = 30 * time.Second
	DefaultPingTimeout  = 10 * time.Second
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

// WithPing configures server-side WebSocket pings emitted by AttachWebSocket.
// interval controls how often pings are sent (0 disables pings); timeout is
// the maximum time to wait for a pong before tearing the connection down.
func WithPing(interval, timeout time.Duration) Option {
	return func(m *manager) {
		m.pingInterval = interval
		m.pingTimeout = timeout
	}
}
