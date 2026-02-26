package app

import (
	"os"
	"time"

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

func WithShutdownTimeout(timeout time.Duration) Option {
	return func(m *manager) {
		m.lc.shutdownTimeout = timeout
	}
}

func WithMonitorInterval(interval time.Duration) Option {
	return func(m *manager) {
		m.monitor.interval = interval
	}
}

func WithRestartPolicy(maxRetries int) Option {
	return func(m *manager) {
		m.monitor.maxRetries = maxRetries
	}
}

func WithRestartDelay(delay time.Duration) Option {
	return func(m *manager) {
		m.monitor.restartDelay = delay
	}
}

func WithSignalHandler(sig os.Signal, handler func()) Option {
	return func(m *manager) {
		m.signals[sig] = handler
	}
}
