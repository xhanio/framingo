package wsutil

import (
	"time"

	"github.com/xhanio/framingo/pkg/utils/log"
)

// Default ping settings Bridge uses when WithPing is not given.
// WithPing(0, ...) disables server-side pings.
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

// setDefaults fills in what the options left unset. The ping settings are
// not here: newBridge seeds them before the options, since a zero interval
// means pings are off.
func (b *bridge) setDefaults() {
	if b.log == nil {
		b.log = log.Default
	}
}

// WithLogger sets the logger a session's opening, closing and errors go to;
// log.Default when not given.
func WithLogger(logger log.Logger) Option {
	return func(b *bridge) {
		b.log = logger
	}
}

// WithPing configures the server-side WebSocket pings. interval controls how
// often pings are sent (0 disables pings); timeout is the maximum time to wait
// for a pong before tearing the connection down.
func WithPing(interval, timeout time.Duration) Option {
	return func(b *bridge) {
		b.pingInterval = interval
		b.pingTimeout = timeout
	}
}
