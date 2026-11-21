package supervisor

import (
	"context"
	"time"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
)

// The operations on one service, by hand or by the monitor. Each takes the op
// lock, and tells the service's dependents what it has become.

func (c *controller) Init(ctx context.Context, service common.Service) error {
	c.op.Lock()
	defer c.op.Unlock()
	return c.init(ctx, service)
}

// Start refuses a service whose Init has not succeeded.
func (c *controller) Start(ctx context.Context, service common.Service) error {
	c.op.Lock()
	defer c.op.Unlock()
	if !c.initialized(service) {
		return errors.Conflict.Newf("service %s is not initialized", service.Name())
	}
	err := c.start(ctx, service)
	if isDaemon(service) || err != nil {
		c.reconcile(ctx, service, stateAfter(err))
	}
	return err
}

func (c *controller) Stop(ctx context.Context, service common.Service) error {
	c.op.Lock()
	defer c.op.Unlock()
	err := c.stop(ctx, service)
	if isDaemon(service) {
		c.reconcile(ctx, service, common.DependencyStopped)
	}
	return err
}

// Restart cycles the service regardless of state, resurrecting even a stopped one.
func (c *controller) Restart(ctx context.Context, service common.Service) error {
	c.op.Lock()
	defer c.op.Unlock()
	return c.restart(ctx, service)
}

// RestartIfRunning re-checks Stopped under the lock so a stop that landed after
// the sweep's snapshot is not resurrected.
func (c *controller) RestartIfRunning(ctx context.Context, service common.Service) error {
	c.op.Lock()
	defer c.op.Unlock()
	if stat := c.stats.snapshot(service.Name()); stat != nil && stat.Stopped {
		c.log.Infof("skipping restart of %s: stopped", service.Name())
		return nil
	}
	return c.restart(ctx, service)
}

// restart cycles service — Stop within the stop policy, Init, Start — and
// tells its dependents each state. While a dependency does not run it leaves
// service as it is: the restart could not hold, and is not counted against it.
func (c *controller) restart(ctx context.Context, service common.Service) error {
	name := service.Name()
	if err := c.depsRunning(service); err != nil {
		return errors.Wrapf(err, "service %s not restarted", name)
	}
	if stat := c.stats.snapshot(name); stat != nil {
		c.log.Infof("restarting service %s (attempt %d)", name, stat.Restarts+1)
	}
	stopCtx, cancel := c.stopContext(ctx)
	if err := c.stop(stopCtx, service); err != nil {
		c.log.Errorf("failed to stop service %s for restart: %s", name, err)
	}
	cancel()
	// stopped for the restart, not for good: a failed one is the monitor's to retry
	c.stats.update(name, func(stat *entity.SupervisorStats) {
		stat.Stopped = false
	})
	// dependents hear the stop even of a service with no Stop: its Init replaces what it holds
	c.reconcile(ctx, service, common.DependencyStopped)
	err := c.init(ctx, service)
	if err == nil {
		err = c.start(ctx, service)
	}
	c.recordRestart(name, err)
	c.reconcile(ctx, service, stateAfter(err))
	if err != nil {
		return err
	}
	c.log.Infof("service %s restarted successfully", name)
	return nil
}

// recordRestart puts a restart's outcome in the healthcheck verdict now, not
// at the next sweep.
func (c *controller) recordRestart(name string, err error) {
	c.stats.update(name, func(stat *entity.SupervisorStats) {
		stat.Restarts++
		stat.RestartedAt = time.Now()
		stat.HealthcheckErr = err
	})
}
