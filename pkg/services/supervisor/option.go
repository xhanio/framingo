package supervisor

import (
	"time"

	"github.com/xhanio/framingo/pkg/types/entity"
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

// WithStopPolicy sets how Stop ends: timeout bounds the whole shutdown,
// 0 (the default) waits indefinitely.
func WithStopPolicy(timeout time.Duration) Option {
	return func(m *manager) {
		m.c.stopPolicy = entity.SupervisorStopPolicy{
			Timeout: timeout,
		}
	}
}

// WithMonitorPolicy sets the health monitor's shape: interval is the sweep
// cadence (0, the default, disables monitoring), maxRetries bounds
// in-process restarts per service (0 none - a liveness failure escalates
// immediately, n up to n, -1 unlimited), and restartDelay pauses before
// each restart attempt.
func WithMonitorPolicy(interval time.Duration, maxRetries int, restartDelay time.Duration) Option {
	return func(m *manager) {
		m.monitor.policy = entity.SupervisorMonitorPolicy{
			Interval:     interval,
			MaxRetries:   maxRetries,
			RestartDelay: restartDelay,
		}
	}
}

// WithInitPolicy sets how a service's init turn retries - waiting on
// dependencies to become ready, or re-running its own failed Init.
// maxRetries bounds the attempts: 0 disables retries (a single pass, the
// default), n > 0 allows n retries after the first attempt, -1 retries
// until the Init context is canceled. The wait between attempts starts at
// delay (1s when unset), doubles per attempt, and caps at maxDelay; when
// maxDelay <= delay the wait stays fixed.
func WithInitPolicy(maxRetries int, delay, maxDelay time.Duration) Option {
	return func(m *manager) {
		m.c.initPolicy = entity.SupervisorInitPolicy{
			MaxRetries: maxRetries,
			Delay:      delay,
			MaxDelay:   maxDelay,
		}
	}
}
