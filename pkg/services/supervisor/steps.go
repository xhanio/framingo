package supervisor

import (
	"context"
	"time"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/confutil"
)

// The steps of one service. Each records what it did in the service's stats;
// none takes the op lock, which its caller holds.

func isDaemon(service common.Service) bool {
	_, ok := service.(common.Daemon)
	return ok
}

func isInitializable(service common.Service) bool {
	_, ok := service.(common.Initializable)
	return ok
}

// init runs service's Init with the config in ctx; a service with no Init is
// initialized as it is.
func (c *controller) init(ctx context.Context, service common.Service) error {
	name := service.Name()
	svc, ok := service.(common.Initializable)
	if !ok {
		c.stats.update(name, func(stat *entity.SupervisorStats) {
			stat.Initialized = true
			stat.Ready = true
		})
		return nil
	}
	c.log.Debugf("initializing %s", name)
	if c.config != nil {
		ctx = confutil.WrapContext(ctx, c.config)
	}
	at := time.Now()
	c.stats.update(name, func(stat *entity.SupervisorStats) {
		stat.InitializedAt = at
	})
	err := svc.Init(ctx)
	c.stats.update(name, func(stat *entity.SupervisorStats) {
		stat.InitDuration = time.Since(at)
		stat.Initialized = err == nil
		stat.InitializationErr = err
		stat.Ready = err == nil
	})
	return err
}

// start starts service, its Start bounded by the start policy, once every
// dependency runs: one that does not gates it, recorded as its start error,
// and nothing runs. A service with no Start has nothing to start.
func (c *controller) start(ctx context.Context, service common.Service) error {
	name := service.Name()
	if err := c.depsRunning(service); err != nil {
		c.stats.update(name, func(stat *entity.SupervisorStats) {
			stat.Started = false
			stat.Stopped = false // meant to run: the monitor restarts it once its dependencies do
			stat.StartErr = err
			stat.Ready = false
		})
		return err
	}
	svc, ok := service.(common.Daemon)
	if !ok {
		c.stats.update(name, func(stat *entity.SupervisorStats) {
			stat.StartErr = nil
		})
		return nil
	}
	ctx, cancel := c.startContext(ctx)
	defer cancel()
	c.log.Debugf("starting %s", name)
	at := time.Now()
	c.stats.update(name, func(stat *entity.SupervisorStats) {
		stat.Started = true
		stat.Stopped = false
		stat.StartedAt = at
	})
	err := svc.Start(ctx)
	c.stats.update(name, func(stat *entity.SupervisorStats) {
		stat.StartErr = err
		stat.StartDuration = time.Since(at)
		stat.Ready = err == nil
		if err == nil {
			stat.ReconcileErr = nil // a start registers everything again
		}
	})
	return err
}

// stop stops service within ctx; a service with no Stop has nothing to stop.
func (c *controller) stop(ctx context.Context, service common.Service) error {
	svc, ok := service.(common.Daemon)
	if !ok {
		return nil
	}
	name := service.Name()
	c.log.Debugf("stopping %s", name)
	at := time.Now()
	c.stats.update(name, func(stat *entity.SupervisorStats) {
		stat.Stopped = true
		stat.Ready = false
		stat.StoppedAt = at
	})
	err := svc.Stop(ctx)
	c.stats.update(name, func(stat *entity.SupervisorStats) {
		stat.StopErr = err
		stat.StopDuration = time.Since(at)
	})
	return err
}

// startContext: the start policy's timeout bounds each Start, and each Reconcile.
func (c *controller) startContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.startPolicy.Timeout > 0 {
		return context.WithTimeout(ctx, c.startPolicy.Timeout)
	}
	return context.WithCancel(ctx)
}

// stopContext: past the stop policy's timeout, the stops left get an ended
// context, forcing them.
func (c *controller) stopContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.stopPolicy.Timeout > 0 {
		return context.WithTimeout(ctx, c.stopPolicy.Timeout)
	}
	return context.WithCancel(ctx)
}
