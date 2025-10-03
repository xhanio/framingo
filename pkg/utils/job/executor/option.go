package executor

import (
	"sync"
	"time"
)

type Option func(*executor)

type retryOptions struct {
	Attempts int
	Delay    time.Duration

	sync.RWMutex
	attempted uint
	errs      []error
}

// WithRetry configures the executor to retry failed job executions.
// The retry mechanism applies to each Start() call independently.
//
// Parameters:
//   - attempts: number of retry attempts (must be > 0)
//   - interval: delay between retry attempts
//
// Note: WithRetry is compatible with Once(). When both are used:
//   - Once() restricts the number of successful Start() calls to one
//   - WithRetry() allows retries within that single Start() call
//
// Example:
//
//	je := New(job, Once(), WithRetry(3, 1*time.Second))
//	je.Start(...) // Will retry up to 3 times on failure
//	je.Start(...) // Will return error "job can only start once" if first call succeeded
func WithRetry(attempts int, interval time.Duration) Option {
	return func(e *executor) {
		if attempts <= 0 {
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

// Once configures the executor to allow only a single successful Start() call.
// After the first successful execution, subsequent Start() calls will return an error.
//
// Note: Once() is compatible with WithRetry(). When both are used:
//   - The first Start() call can retry on failure (if WithRetry is configured)
//   - Only one successful Start() execution is allowed
//   - Once a Start() call succeeds, no further Start() calls are permitted
//
// This is useful for idempotent initialization tasks that should only complete once
// but may need retries due to transient failures.
//
// Example:
//
//	je := New(job, Once(), WithRetry(3, 1*time.Second))
//	je.Start(...) // May retry up to 3 times on failure
//	je.Start(...) // Returns error if previous call succeeded
func Once() Option {
	return func(e *executor) {
		e.once = true
	}
}
