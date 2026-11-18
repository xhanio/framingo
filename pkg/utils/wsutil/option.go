package wsutil

import (
	"time"

	"github.com/xhanio/framingo/pkg/utils/log"
)

const (
	DefaultPingInterval = 30 * time.Second
	DefaultPingTimeout  = 10 * time.Second
)

type Option func(*bridge)

func (b *bridge) applyOptions(opts ...Option) {
	for _, opt := range opts {
		opt(b)
	}
}

// ping settings are seeded by newBridge instead, since a zero interval means off
func (b *bridge) setDefaults() {
	if b.log == nil {
		b.log = log.Default
	}
}

func WithLogger(logger log.Logger) Option {
	return func(b *bridge) {
		b.log = logger
	}
}

// WithPing: interval 0 disables pings; a pong missing past timeout tears the session down.
func WithPing(interval, timeout time.Duration) Option {
	return func(b *bridge) {
		b.pingInterval = interval
		b.pingTimeout = timeout
	}
}
