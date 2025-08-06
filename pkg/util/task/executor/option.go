package executor

import (
	"sync"
	"time"
)

type Option func(*executor)

type retryOptions struct {
	Attempts uint
	Delay    time.Duration

	sync.RWMutex
	attempted uint
	errs      []error
}

func WithRetry(attempts uint, interval time.Duration) Option {
	return func(e *executor) {
		if attempts == 0 {
			return
		}
		e.retry = &retryOptions{
			Attempts: attempts,
			Delay:    interval,

			errs: make([]error, attempts),
		}
	}
}

type timeoutOptions struct {
	Duration time.Duration
}

func WithTimeout(timeout time.Duration) Option {
	return func(e *executor) {
		if timeout <= 0 {
			return
		}
		e.timeout = &timeoutOptions{
			Duration: timeout,
		}
	}
}

func NoTimeout() Option {
	return WithTimeout(-1)
}

type cooldownOptions struct {
	Duration time.Duration

	sync.RWMutex
	endedAt time.Time
}

func WithCooldown(cooldown time.Duration) Option {
	return func(e *executor) {
		if cooldown <= 0 {
			return
		}
		e.cooldown = &cooldownOptions{
			Duration: cooldown,
		}
	}
}

func Once() Option {
	return func(e *executor) {
		e.once = true
	}
}
