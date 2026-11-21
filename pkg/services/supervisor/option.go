package supervisor

import (
	"time"

	"github.com/xhanio/framingo/pkg/types/entity"
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

// WithStartPolicy bounds each service's Start, at the first start and at each
// restart; past timeout that Start's context ends, and a Start that honors it
// fails. 0 leaves it to the context the supervisor's Start, or the restart,
// was given.
func WithStartPolicy(timeout time.Duration) Option {
	return func(m *manager) {
		m.c.startPolicy = entity.SupervisorStartPolicy{
			Timeout: timeout,
		}
	}
}

// WithStopPolicy bounds shutdown and each restart's stop; past timeout remaining
// stops are forced. 0 leaves it to Stop's context.
func WithStopPolicy(timeout time.Duration) Option {
	return func(m *manager) {
		m.c.stopPolicy = entity.SupervisorStopPolicy{
			Timeout: timeout,
		}
	}
}

// WithMonitorPolicy: interval 0 disables monitoring; maxRetries 0 escalates a
// liveness failure at once, -1 restarts forever.
func WithMonitorPolicy(interval time.Duration, maxRetries int, restartDelay time.Duration) Option {
	return func(m *manager) {
		m.monitor.policy = entity.SupervisorMonitorPolicy{
			Interval:     interval,
			MaxRetries:   maxRetries,
			RestartDelay: restartDelay,
		}
	}
}

// WithInitPolicy retries init turns (dependency readiness and own Init):
// maxRetries 0 single pass, -1 until ctx ends; backoff doubles from delay
// (default 1s) up to maxDelay, fixed when maxDelay <= delay.
func WithInitPolicy(maxRetries int, delay, maxDelay time.Duration) Option {
	return func(m *manager) {
		m.c.initPolicy = entity.SupervisorInitPolicy{
			MaxRetries: maxRetries,
			Delay:      delay,
			MaxDelay:   maxDelay,
		}
	}
}
