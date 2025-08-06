package executor

import (
	"context"
	"time"

	"github.com/avast/retry-go/v4"

	"xhanio/framingo/pkg/util/errors"
	"xhanio/framingo/pkg/util/task"
)

var _ Executor = (*executor)(nil)

type executor struct {
	t        task.Task
	once     bool
	timeout  *timeoutOptions
	retry    *retryOptions
	cooldown *cooldownOptions
}

func New(t task.Task, opts ...Option) Executor {
	return newExecuter(t, opts...)
}

func newExecuter(t task.Task, opts ...Option) *executor {
	e := &executor{
		t: t,
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

func (e *executor) run(ctx context.Context) error {
	if e.timeout != nil {
		timeoutctx, cancel := context.WithTimeout(ctx, e.timeout.Duration)
		defer cancel()
		e.t.Start(timeoutctx)
	} else {
		e.t.Start(ctx)
	}
	e.t.Wait()
	return e.t.Err()
}

func (e *executor) Start(ctx context.Context) error {
	if e.t.IsDone() {
		if e.once {
			return errors.Conflict.Newf("task can only start once")
		}
		if e.cooldown != nil && time.Now().Before(e.cooldown.endedAt) {
			return errors.Conflict.Newf("task is still in cooldown, %s left", time.Until(e.cooldown.endedAt).Round(time.Second).String())
		}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if !e.once && e.retry != nil {
		// with retries
		return retry.Do(
			func() error {
				return e.run(ctx)
			},
			retry.Attempts(e.retry.Attempts),
			retry.Delay(e.retry.Delay),
			retry.OnRetry(func(n uint, err error) {
				e.retry.attempted = n + 1
				e.retry.errs[n] = err
			}),
		)
	}
	return e.run(ctx)
}

func (e *executor) Stop(wait bool) error {
	canceling := e.t.Cancel()
	if canceling && wait {
		e.t.Wait()
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
