package executor

import (
	"sync"
	"time"

	"github.com/xhanio/framingo/pkg/utils/job"
)

type Option func(*executor)

func (e *executor) applyOptions(opts ...Option) {
	for _, opt := range opts {
		opt(e)
	}
}

type retryOptions struct {
	Retries int
	Delay   time.Duration

	sync.RWMutex
	retried uint
}

// Construction options: fixed for the instance's life, set at New (design/config.md).

// WithRetry applies per Start(); retries <= 0 is ignored, and interval doubles after each retry.
func WithRetry(retries int, interval time.Duration) Option {
	return func(e *executor) {
		if retries <= 0 {
			return
		}
		e.retry = &retryOptions{
			Retries: retries,
			Delay:   interval,
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

func OnComplete(fn func(job.Job)) Option {
	return func(e *executor) {
		e.onComplete = fn
	}
}
