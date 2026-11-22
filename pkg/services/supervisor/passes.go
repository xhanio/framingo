package supervisor

import (
	"context"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
)

// The steps of the whole graph, in dependency order — stops in reverse.

func (c *controller) InitAll(ctx context.Context) error {
	c.op.Lock()
	defer c.op.Unlock()
	return c.initAll(ctx)
}

func (c *controller) StartAll(ctx context.Context) error {
	c.op.Lock()
	defer c.op.Unlock()
	return c.startAll(ctx)
}

func (c *controller) StopAll(ctx context.Context) error {
	c.op.Lock()
	defer c.op.Unlock()
	return c.stopAll(ctx)
}

// tally counts a pass and collects its failures, each named for its service.
type tally struct {
	total, failed int
	errs          []error
}

func (t *tally) add(service common.Service, counted bool, err error) {
	if counted {
		t.total++
	}
	if err != nil {
		t.failed++
		t.errs = append(t.errs, errors.Wrapf(err, "service %s", service.Name()))
	}
}

func (t *tally) err() error {
	return errors.Combine(t.errs...)
}

func (c *controller) initAll(ctx context.Context) error {
	c.log.Info("initializing services...")
	policy := c.passPolicy()
	var t tally
	for _, service := range c.services {
		ran, err := c.initTurn(ctx, service, policy)
		t.add(service, ran, err)
	}
	c.booted = true
	c.log.Infof("%d services initialized, %d failed", t.total, t.failed)
	return t.err()
}

// passPolicy is the init policy of this pass: the first Init's waits, and a
// later one — a reload's, a whole restart's, of a process that was serving —
// makes one pass, since what fails then is the change, which no wait fixes.
func (c *controller) passPolicy() entity.SupervisorInitPolicy {
	if c.booted {
		return entity.SupervisorInitPolicy{}
	}
	return c.initPolicy
}

// initTurn runs service's Init once its dependencies are initialized. Under a
// policy that waits, a dependency not yet ready and the service's own failing
// Init are tried again, with backoff; a dependency whose Init failed fails the
// turn at once. ran reports whether an Init ran.
func (c *controller) initTurn(ctx context.Context, service common.Service, policy entity.SupervisorInitPolicy) (ran bool, err error) {
	name := service.Name()
	reached := false // the dependencies let a try through
	var last error
	err = retry.Do(func() error {
		blocked, transient := c.depsInitialized(ctx, service, policy)
		if blocked != nil {
			last = blocked
			if !transient {
				return retry.Unrecoverable(blocked)
			}
			return blocked
		}
		reached = true
		last = c.init(ctx, service)
		return last
	}, c.retryOptions(ctx, name, policy)...)
	if err != nil && !reached {
		if last == nil {
			last = err // canceled before the first try
		}
		c.stats.update(name, func(stat *entity.SupervisorStats) {
			stat.InitializationErr = last
		})
		return false, last
	}
	return reached && isInitializable(service), err
}

// retryOptions: MaxRetries 0 is one try, n is n+1 tries, -1 tries until ctx
// ends; the delay, 1s by default, doubles up to MaxDelay, or stays fixed when
// MaxDelay does not exceed it.
func (c *controller) retryOptions(ctx context.Context, name string, policy entity.SupervisorInitPolicy) []retry.Option {
	attempts := uint(1)
	switch {
	case policy.MaxRetries > 0:
		attempts = uint(policy.MaxRetries) + 1
	case policy.MaxRetries < 0:
		attempts = 0 // unlimited, bounded by ctx
	}
	delay := policy.Delay
	if delay <= 0 {
		delay = time.Second
	}
	opts := []retry.Option{
		retry.Context(ctx),
		retry.Attempts(attempts),
		retry.Delay(delay),
		retry.LastErrorOnly(true),
		retry.OnRetry(func(n uint, err error) {
			if n+1 == attempts { // retry-go reports the last try's failure too, no retry after it
				return
			}
			c.log.Warnf("init of %s blocked, retrying: %s", name, err)
		}),
	}
	if policy.MaxDelay > delay {
		return append(opts, retry.DelayType(retry.BackOffDelay), retry.MaxDelay(policy.MaxDelay))
	}
	return append(opts, retry.DelayType(retry.FixedDelay))
}

// startAll starts each service whose Init succeeded and whose dependencies
// run, as an init turn fails a dependent fast; the rest are the monitor's to
// restart.
func (c *controller) startAll(ctx context.Context) error {
	c.log.Info("starting services...")
	var t tally
	for _, service := range c.services {
		if !c.initialized(service) {
			// its init error is the record; not stopped, so the monitor restarts it
			c.stats.update(service.Name(), func(stat *entity.SupervisorStats) {
				stat.Stopped = false
			})
			continue
		}
		err := c.start(ctx, service)
		t.add(service, isDaemon(service) || err != nil, err)
	}
	c.log.Infof("%d services started, %d failed", t.total, t.failed)
	return t.err()
}

// stopAll stops dependents before their dependencies, the whole pass bounded
// by the stop policy.
func (c *controller) stopAll(ctx context.Context) error {
	ctx, cancel := c.stopContext(ctx)
	defer cancel()
	c.log.Info("stopping services...")
	var t tally
	for i := len(c.services) - 1; i >= 0; i-- {
		service := c.services[i]
		t.add(service, isDaemon(service), c.stop(ctx, service))
	}
	if ctx.Err() != nil {
		c.log.Warnf("shutdown ended before the services did: the stops after it were forced")
	}
	c.log.Infof("%d services stopped, %d failed", t.total, t.failed)
	return t.err()
}
