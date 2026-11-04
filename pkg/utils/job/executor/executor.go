package executor

import (
	"context"

	"github.com/avast/retry-go/v4"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/utils/job"
)

var _ Executor = (*executor)(nil)

type executor struct {
	j          job.Job
	timeout    *timeoutOptions
	retry      *retryOptions
	onComplete func(job.Job)
}

func New(j job.Job, opts ...Option) Executor {
	return newExecuter(j, opts...)
}

func newExecuter(j job.Job, opts ...Option) *executor {
	e := &executor{
		j: j,
	}
	e.applyOptions(opts...)
	return e
}

func (e *executor) Reset() {
	if e.retry != nil {
		e.retry.Lock()
		e.retry.retried = 0
		e.retry.Unlock()
	}
}

func (e *executor) run(ctx context.Context, params any) (retryable bool, err error) {
	var started bool
	if e.timeout != nil {
		timeoutctx, cancel := context.WithTimeout(ctx, e.timeout.Duration)
		defer cancel()
		started = e.j.Run(timeoutctx, params)
	} else {
		started = e.j.Run(ctx, params)
	}
	if !started {
		// retrying would only run it again once the earlier start ends
		return false, errors.Conflict.Newf("job %s is still pending", e.j.ID())
	}
	e.j.Wait()
	err = e.j.Err()
	if err != nil && (e.j.IsState(job.StateCanceled) || ctx.Err() != nil) {
		return false, err
	}
	return true, err
}

func (e *executor) Start(ctx context.Context, params any) error {
	if ctx == nil {
		ctx = context.Background()
	}

	var err error
	if e.retry != nil {
		var runs uint
		err = retry.Do(
			func() error {
				e.retry.Lock()
				e.retry.retried = runs
				e.retry.Unlock()
				runs++
				retryable, err := e.run(ctx, params)
				if !retryable {
					return retry.Unrecoverable(err)
				}
				return err
			},
			retry.Context(ctx),
			// retry-go's attempts include the first run
			retry.Attempts(uint(e.retry.Retries)+1),
			retry.Delay(e.retry.Delay),
		)
	} else {
		_, err = e.run(ctx, params)
	}

	if e.onComplete != nil {
		e.onComplete(e.j)
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

func (e *executor) Stats() *Stats {
	stat := &Stats{}
	if e.retry != nil {
		e.retry.RLock()
		stat.Retries = e.retry.retried
		e.retry.RUnlock()
	}
	return stat
}
