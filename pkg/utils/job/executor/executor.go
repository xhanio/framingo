package executor

import (
	"context"
	"time"

	"github.com/avast/retry-go/v4"

	"github.com/xhanio/framingo/pkg/utils/errors"
	"github.com/xhanio/framingo/pkg/utils/job"
)

var _ Executor = (*executor)(nil)

type executor struct {
	j        job.Job
	once     bool
	timeout  *timeoutOptions
	retry    *retryOptions
	cooldown *cooldownOptions
}

func New(j job.Job, opts ...Option) Executor {
	return newExecuter(j, opts...)
}

func newExecuter(j job.Job, opts ...Option) *executor {
	e := &executor{
		j: j,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func (e *executor) Reset() {
	if e.retry != nil {
		e.retry.Lock()
		e.retry.attempted = 0
		e.retry.errs = make([]error, e.retry.Attempts)
		e.retry.Unlock()
	}
	if e.cooldown != nil {
		e.cooldown.Lock()
		e.cooldown.endedAt = time.Time{}
		e.cooldown.Unlock()
	}
}

func (e *executor) run(ctx context.Context, params any) error {
	var started bool
	if e.timeout != nil {
		timeoutctx, cancel := context.WithTimeout(ctx, e.timeout.Duration)
		defer cancel()
		started = e.j.Run(timeoutctx, params)
	} else {
		started = e.j.Run(ctx, params)
	}
	if !started {
		return errors.Conflict.Newf("job %s is still pending", e.j.ID())
	}
	e.j.Wait()
	return e.j.Err()
}

func (e *executor) Start(ctx context.Context, params any) error {
	if e.j.IsDone() {
		if e.once {
			return errors.Conflict.Newf("job can only start once")
		}
		if e.cooldown != nil && time.Now().Before(e.cooldown.endedAt) {
			return errors.Conflict.Newf("job is still in cooldown, %s left", time.Until(e.cooldown.endedAt).Round(time.Second).String())
		}
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var err error
	if e.retry != nil {
		// with retries - works regardless of Once setting
		// Once means "can only start once", retry means "retry within this execution"
		err = retry.Do(
			func() error {
				return e.run(ctx, params)
			},
			retry.Attempts(uint(e.retry.Attempts)),
			retry.Delay(e.retry.Delay),
			retry.OnRetry(func(n uint, err error) {
				e.retry.attempted = n + 1
				e.retry.errs[n] = err
			}),
		)
	} else {
		err = e.run(ctx, params)
	}

	// Set cooldown after job completes
	if e.cooldown != nil {
		e.cooldown.Lock()
		e.cooldown.endedAt = time.Now().Add(e.cooldown.Duration)
		e.cooldown.Unlock()
	}

	return err
}

func (e *executor) Stop(wait bool) error {
	canceling := e.j.Cancel()
	if canceling && wait {
		e.j.Wait()
	}
	return nil
}

func (e *executor) isCooling() (time.Duration, bool) {
	if e.cooldown == nil {
		return 0, false
	}
	e.cooldown.RLock()
	defer e.cooldown.RUnlock()
	d := time.Until(e.cooldown.endedAt)
	return d, d > 0
}

func (e *executor) Stats() *Stats {
	cooldown, _ := e.isCooling()
	stat := &Stats{
		Cooldown: cooldown,
	}
	if e.retry != nil {
		stat.Retries = e.retry.attempted
	}
	return stat
}
