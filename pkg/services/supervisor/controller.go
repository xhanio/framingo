package supervisor

import (
	"context"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/structs/graph"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/confutil"
	"github.com/xhanio/framingo/pkg/utils/log"
)

type controller struct {
	log        log.Logger
	config     confutil.Config
	initPolicy entity.SupervisorInitPolicy
	stopPolicy entity.SupervisorStopPolicy

	graph    graph.Graph[common.Service]
	services []common.Service
	stats    *statsStore

	// op serializes restarts, manual ops and shutdown; exported methods take it,
	// unexported ones run under a caller's lock or before the monitor starts.
	op sync.Mutex
}

func newController(config confutil.Config) *controller {
	return &controller{
		config: config,
		graph:  graph.New[common.Service](),
		stats:  newStatsStore(),
	}
}

// register refuses a second instance under a taken name (the graph keys on
// names); the same instance again is a no-op. Wiring-time only: c.services is
// read unlocked.
func (c *controller) register(service common.Service) error {
	if holder := c.stats.track(service); holder != nil {
		if holder != service {
			return errors.Conflict.Newf(
				"service name %s is taken by another instance (%T, now %T): give each its own name, such as with WithName",
				service.Name(), holder, service)
		}
	} else if len(c.services) > 0 {
		c.services = append(c.services, service)
	}
	c.graph.Add(service)
	return nil
}

func (c *controller) addDependency(service, dep common.Service) error {
	if err := c.register(dep); err != nil {
		return err
	}
	c.graph.Add(service, dep)
	return nil
}

func (c *controller) topoSort() error {
	err := c.graph.TopoSort()
	if err != nil {
		return errors.Wrap(err)
	}
	c.services = c.graph.Nodes()
	return nil
}

func (c *controller) find(name string) common.Service {
	for _, service := range c.services {
		if service.Name() == name {
			return service
		}
	}
	return nil
}

func (c *controller) init(ctx context.Context, service common.Service) (bool, error) {
	name := service.Name()
	svc, ok := service.(common.Initializable)
	if !ok {
		c.stats.update(name, func(stat *entity.SupervisorStats) {
			stat.Initialized = true
			stat.Ready = true
		})
		return false, nil
	}
	c.log.Debugf("initializing %s", name)
	if c.config != nil {
		ctx = confutil.WrapContext(ctx, c.config)
	}
	initializedAt := time.Now()
	c.stats.update(name, func(stat *entity.SupervisorStats) {
		stat.InitializedAt = initializedAt
	})
	err := svc.Init(ctx)
	c.stats.update(name, func(stat *entity.SupervisorStats) {
		stat.InitDuration = time.Since(initializedAt)
		stat.Initialized = err == nil
		stat.InitializationErr = err
		stat.Ready = err == nil
	})
	return true, err
}

func (c *controller) start(service common.Service) (bool, error) {
	svc, ok := service.(common.Daemon)
	if !ok {
		return false, nil
	}
	name := service.Name()
	c.log.Debugf("starting %s", name)
	startedAt := time.Now()
	c.stats.update(name, func(stat *entity.SupervisorStats) {
		stat.Started = true
		stat.Stopped = false
		stat.StartedAt = startedAt
	})
	err := svc.Start(context.Background())
	c.stats.update(name, func(stat *entity.SupervisorStats) {
		stat.StartErr = err
		stat.StartDuration = time.Since(startedAt)
		stat.Ready = err == nil
	})
	return true, err
}

func (c *controller) stop(ctx context.Context, service common.Service) (bool, error) {
	svc, ok := service.(common.Daemon)
	if !ok {
		return false, nil
	}
	name := service.Name()
	c.log.Debugf("stopping %s", name)
	stoppedAt := time.Now()
	c.stats.update(name, func(stat *entity.SupervisorStats) {
		stat.Stopped = true
		stat.Ready = false
		stat.StoppedAt = stoppedAt
	})
	err := svc.Stop(ctx)
	c.stats.update(name, func(stat *entity.SupervisorStats) {
		stat.StopErr = err
		stat.StopDuration = time.Since(stoppedAt)
	})
	return true, err
}

func (c *controller) Init(ctx context.Context, service common.Service) error {
	c.op.Lock()
	defer c.op.Unlock()
	_, err := c.init(ctx, service)
	return err
}

func (c *controller) Start(service common.Service) error {
	c.op.Lock()
	defer c.op.Unlock()
	_, err := c.start(service)
	return err
}

func (c *controller) Stop(ctx context.Context, service common.Service) error {
	c.op.Lock()
	defer c.op.Unlock()
	_, err := c.stop(ctx, service)
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

func (c *controller) restart(ctx context.Context, service common.Service) error {
	name := service.Name()
	if stat := c.stats.snapshot(name); stat != nil {
		c.log.Infof("restarting service %s (attempt %d)", name, stat.Restarts+1)
	}
	if svc, ok := service.(common.Daemon); ok {
		stopCtx, cancel := c.stopContext(ctx)
		err := svc.Stop(stopCtx)
		cancel()
		if err != nil {
			c.log.Errorf("failed to stop service %s for restart: %s", name, err)
		}
		c.stats.update(name, func(stat *entity.SupervisorStats) {
			stat.Stopped = false
		})
	}
	// record the outcome in the healthcheck verdict now, not at the next sweep
	restarted := func(err error) {
		c.stats.update(name, func(stat *entity.SupervisorStats) {
			stat.Restarts++
			stat.RestartedAt = time.Now()
			stat.HealthcheckErr = err
		})
	}
	if _, err := c.init(ctx, service); err != nil {
		restarted(err)
		return err
	}
	if _, err := c.start(service); err != nil {
		restarted(err)
		return err
	}
	restarted(nil)
	c.log.Infof("service %s restarted successfully", name)
	return nil
}

func (c *controller) initAll(ctx context.Context) error {
	c.log.Info("initializing services...")
	var errs []error
	var total, failed int
	for _, service := range c.services {
		ok, err := c.initTurn(ctx, service)
		if err != nil {
			failed++
			errs = append(errs, errors.Wrapf(err, "service %s", service.Name()))
		}
		if ok {
			total++
		}
	}
	c.log.Infof("%d services initialized, %d failed", total, failed)
	return errors.Combine(errs...)
}

// initTurn retries transient blockages per the init policy; a dependency that
// failed its own turn fails this one fast. ok reports whether Init itself ran.
func (c *controller) initTurn(ctx context.Context, service common.Service) (bool, error) {
	name := service.Name()
	// retry-go attempts: 0 -> 1, n -> n+1, -1 -> unlimited (bounded by ctx)
	attempts := uint(1)
	switch {
	case c.initPolicy.MaxRetries > 0:
		attempts = uint(c.initPolicy.MaxRetries) + 1
	case c.initPolicy.MaxRetries < 0:
		attempts = 0
	}
	delay := c.initPolicy.Delay
	if delay <= 0 {
		delay = time.Second
	}
	opts := []retry.Option{
		retry.Context(ctx),
		retry.Attempts(attempts),
		retry.Delay(delay),
		retry.LastErrorOnly(true),
		retry.OnRetry(func(n uint, err error) {
			c.log.Warnf("init of %s blocked, retrying: %s", name, err)
		}),
	}
	if c.initPolicy.MaxDelay > delay {
		opts = append(opts, retry.DelayType(retry.BackOffDelay), retry.MaxDelay(c.initPolicy.MaxDelay))
	} else {
		opts = append(opts, retry.DelayType(retry.FixedDelay))
	}
	attempted := false
	var blocked error
	err := retry.Do(func() error {
		var transient bool
		if blocked, transient = c.depsInitReady(ctx, service); blocked != nil {
			if !transient {
				return retry.Unrecoverable(blocked)
			}
			return blocked
		}
		ok, initErr := c.init(ctx, service)
		attempted = attempted || ok
		blocked = initErr
		return initErr
	}, opts...)
	if err != nil && !attempted {
		if blocked == nil {
			blocked = err // canceled before the first attempt
		}
		c.stats.update(name, func(stat *entity.SupervisorStats) {
			stat.InitializationErr = blocked
		})
		return false, blocked
	}
	return attempted, err
}

// depsInitReady: a failing Ready probe is transient, a dependency's failed init
// turn is not; without an init policy it never probes.
func (c *controller) depsInitReady(ctx context.Context, service common.Service) (blocked error, transient bool) {
	for _, dep := range service.Dependencies() {
		if dep == nil {
			continue // an optional dependency the app did not wire: nothing to wait for
		}
		if stat := c.stats.snapshot(dep.Name()); stat != nil && !stat.Initialized {
			c.log.Debugf("%s dependency %s is not initialized", service.Name(), dep.Name())
			return errors.Newf("dependencies not ready"), false
		}
		if c.initPolicy.MaxRetries == 0 {
			continue
		}
		if readiness, ok := dep.(common.Readiness); ok {
			if err := readiness.Ready(ctx); err != nil {
				return errors.Wrapf(err, "dependency %s not ready", dep.Name()), true
			}
		}
	}
	return nil, false
}

func (c *controller) startAll() error {
	c.log.Info("starting services...")
	var errs []error
	var total, failed int
	for _, service := range c.services {
		ok, err := c.start(service)
		if ok {
			if err != nil {
				failed++
				errs = append(errs, errors.Wrapf(err, "service %s", service.Name()))
			}
			total++
		}
	}
	c.log.Infof("%d services started, %d failed", total, failed)
	return errors.Combine(errs...)
}

func (c *controller) StopAll(ctx context.Context) error {
	c.op.Lock()
	defer c.op.Unlock()
	return c.stopAll(ctx)
}

// stopContext: past the stop policy's timeout, remaining stops get an ended context, forcing them.
func (c *controller) stopContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.stopPolicy.Timeout > 0 {
		return context.WithTimeout(ctx, c.stopPolicy.Timeout)
	}
	return context.WithCancel(ctx)
}

func (c *controller) stopAll(ctx context.Context) error {
	ctx, cancel := c.stopContext(ctx)
	defer cancel()
	c.log.Info("stopping services...")
	var errs []error
	var total, failed int
	l := len(c.services)
	for i := l - 1; i > -1; i-- {
		service := c.services[i]
		ok, err := c.stop(ctx, service)
		if ok {
			if err != nil {
				failed++
				errs = append(errs, errors.Wrapf(err, "service %s", service.Name()))
			}
			total++
		}
	}
	if ctx.Err() != nil {
		c.log.Warnf("shutdown ended before the services did: the stops after it were forced")
	}
	c.log.Infof("%d services stopped, %d failed", total, failed)
	return errors.Combine(errs...)
}
