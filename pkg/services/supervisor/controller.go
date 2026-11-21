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
	log         log.Logger
	config      confutil.Config
	initPolicy  entity.SupervisorInitPolicy
	startPolicy entity.SupervisorStartPolicy
	stopPolicy  entity.SupervisorStopPolicy

	graph    graph.Graph[common.Service]
	services []common.Service
	stats    *statsStore
	booted   bool // the first Init pass has run: a later one makes one pass

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

// start starts service once every dependency is running; one that is not
// gates it, recorded as its start error, and nothing runs.
func (c *controller) start(ctx context.Context, service common.Service) (bool, error) {
	name := service.Name()
	if err := c.depsRunning(service); err != nil {
		c.stats.update(name, func(stat *entity.SupervisorStats) {
			stat.Started = false
			stat.Stopped = false // meant to run: the monitor restarts it once its dependencies are
			stat.StartErr = err
			stat.Ready = false
		})
		return true, err
	}
	svc, ok := service.(common.Daemon)
	if !ok {
		c.stats.update(name, func(stat *entity.SupervisorStats) {
			stat.StartErr = nil
		})
		return false, nil
	}
	ctx, cancel := c.startContext(ctx)
	defer cancel()
	c.log.Debugf("starting %s", name)
	startedAt := time.Now()
	c.stats.update(name, func(stat *entity.SupervisorStats) {
		stat.Started = true
		stat.Stopped = false
		stat.StartedAt = startedAt
	})
	err := svc.Start(ctx)
	c.stats.update(name, func(stat *entity.SupervisorStats) {
		stat.StartErr = err
		stat.StartDuration = time.Since(startedAt)
		stat.Ready = err == nil
		if err == nil {
			stat.ReconcileErr = nil // a start registers everything again
		}
	})
	return true, err
}

// depsRunning is the first dependency not running.
func (c *controller) depsRunning(service common.Service) error {
	for _, dep := range service.Dependencies() {
		if dep == nil {
			continue // an optional dependency the app did not wire: nothing to wait for
		}
		if !c.running(dep) {
			return errors.Unavailable.Newf("dependency %s is not running", dep.Name())
		}
	}
	return nil
}

// running: initialized, started, not stopped, and neither its start nor a
// Reconcile since has failed. A service the supervisor does not track counts.
func (c *controller) running(service common.Service) bool {
	stat := c.stats.snapshot(service.Name())
	if stat == nil {
		return true
	}
	_, daemon := service.(common.Daemon)
	return stat.Initialized && stat.StartErr == nil && stat.ReconcileErr == nil && !stat.Stopped && (!daemon || stat.Started)
}

// reconcile tells each running dependent of dep that is Reconcilable what dep
// has become, in dependency order; a dependent that fails is failed, for the
// monitor to restart.
func (c *controller) reconcile(ctx context.Context, dep common.Service, state common.DependencyState) {
	for _, service := range c.services {
		svc, ok := service.(common.Reconcilable)
		if !ok || service.Name() == dep.Name() || !dependsOn(service, dep) || !c.running(service) {
			continue
		}
		name := service.Name()
		rctx, cancel := c.startContext(ctx)
		err := svc.Reconcile(rctx, dep, state)
		cancel()
		if err == nil {
			continue
		}
		err = errors.Wrapf(err, "failed to reconcile with %s %s", dep.Name(), state)
		c.log.Errorf("service %s %s", name, err)
		c.stats.update(name, func(stat *entity.SupervisorStats) {
			if stat.ReconcileErr == nil {
				stat.ReconcileErr = err
			}
			stat.Ready = false
		})
	}
}

// dependsOn by name, as the graph keys: one instance per name
func dependsOn(service, dep common.Service) bool {
	for _, d := range service.Dependencies() {
		if d != nil && d.Name() == dep.Name() {
			return true
		}
	}
	return false
}

// initialized reports whether service's last Init succeeded; one that failed
// says so in its stats, and is not started.
func (c *controller) initialized(service common.Service) bool {
	stat := c.stats.snapshot(service.Name())
	return stat == nil || stat.Initialized
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

func (c *controller) Start(ctx context.Context, service common.Service) error {
	c.op.Lock()
	defer c.op.Unlock()
	if !c.initialized(service) {
		return errors.Conflict.Newf("service %s is not initialized", service.Name())
	}
	ok, err := c.start(ctx, service)
	if ok {
		c.reconcile(ctx, service, started(err))
	}
	return err
}

func (c *controller) Stop(ctx context.Context, service common.Service) error {
	c.op.Lock()
	defer c.op.Unlock()
	ok, err := c.stop(ctx, service)
	if ok {
		c.reconcile(ctx, service, common.DependencyStopped)
	}
	return err
}

// started is the state a start's err leaves its service in.
func started(err error) common.DependencyState {
	if err != nil {
		return common.DependencyFailed
	}
	return common.DependencyRunning
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

// restart leaves service as it is while a dependency is not running: the
// restart could not hold, and is not counted against it.
func (c *controller) restart(ctx context.Context, service common.Service) error {
	name := service.Name()
	if err := c.depsRunning(service); err != nil {
		return errors.Wrapf(err, "service %s not restarted", name)
	}
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
	// dependents hear the stop even of a service with no Stop: its Init replaces what it holds
	c.reconcile(ctx, service, common.DependencyStopped)
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
		c.reconcile(ctx, service, common.DependencyFailed)
		return err
	}
	if _, err := c.start(ctx, service); err != nil {
		restarted(err)
		c.reconcile(ctx, service, common.DependencyFailed)
		return err
	}
	restarted(nil)
	c.reconcile(ctx, service, common.DependencyRunning)
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
	c.booted = true
	return errors.Combine(errs...)
}

// policy is the init policy of this pass: the boot's waits, and a later pass
// — a reload, a whole restart, of a process that was serving — makes one.
func (c *controller) policy() entity.SupervisorInitPolicy {
	if c.booted {
		return entity.SupervisorInitPolicy{}
	}
	return c.initPolicy
}

// initTurn retries transient blockages per the init policy; a dependency that
// failed its own turn fails this one fast. ok reports whether Init itself ran.
func (c *controller) initTurn(ctx context.Context, service common.Service) (bool, error) {
	name := service.Name()
	policy := c.policy()
	// retry-go attempts: 0 -> 1, n -> n+1, -1 -> unlimited (bounded by ctx)
	attempts := uint(1)
	switch {
	case policy.MaxRetries > 0:
		attempts = uint(policy.MaxRetries) + 1
	case policy.MaxRetries < 0:
		attempts = 0
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
			c.log.Warnf("init of %s blocked, retrying: %s", name, err)
		}),
	}
	if policy.MaxDelay > delay {
		opts = append(opts, retry.DelayType(retry.BackOffDelay), retry.MaxDelay(policy.MaxDelay))
	} else {
		opts = append(opts, retry.DelayType(retry.FixedDelay))
	}
	attempted := false
	var blocked error
	err := retry.Do(func() error {
		var transient bool
		if blocked, transient = c.depsInitReady(ctx, service, policy); blocked != nil {
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
func (c *controller) depsInitReady(ctx context.Context, service common.Service, policy entity.SupervisorInitPolicy) (blocked error, transient bool) {
	for _, dep := range service.Dependencies() {
		if dep == nil {
			continue // an optional dependency the app did not wire: nothing to wait for
		}
		if stat := c.stats.snapshot(dep.Name()); stat != nil && !stat.Initialized {
			c.log.Debugf("%s dependency %s is not initialized", service.Name(), dep.Name())
			return errors.Newf("dependencies not ready"), false
		}
		if policy.MaxRetries == 0 {
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

// startAll starts, in dependency order, each service whose Init succeeded and
// whose dependencies are running, as initAll fails a dependent fast; the
// rest are left to the monitor's restarts.
func (c *controller) startAll(ctx context.Context) error {
	c.log.Info("starting services...")
	var errs []error
	var total, failed int
	for _, service := range c.services {
		if !c.initialized(service) {
			// its init error is the record; not stopped, so the monitor restarts it
			c.stats.update(service.Name(), func(stat *entity.SupervisorStats) {
				stat.Stopped = false
			})
			continue
		}
		ok, err := c.start(ctx, service)
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

// startContext: the start policy's timeout bounds each service's Start.
func (c *controller) startContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.startPolicy.Timeout > 0 {
		return context.WithTimeout(ctx, c.startPolicy.Timeout)
	}
	return context.WithCancel(ctx)
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
