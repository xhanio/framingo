package supervisor

import (
	"context"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/spf13/viper"
	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/structs/graph"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/confutil"
	"github.com/xhanio/framingo/pkg/utils/log"
)

type controller struct {
	log        log.Logger
	config     *viper.Viper
	initPolicy entity.SupervisorInitPolicy
	stopPolicy entity.SupervisorStopPolicy

	graph    graph.Graph[common.Service]
	services []common.Service
	stats    *statsStore

	// op serializes service operations - restarts, manual init/start/stop,
	// and the whole shutdown - so stop-init-start cycles never interleave.
	// Exported controller methods take it; unexported ones don't lock and
	// run either under an exported caller's lock or in the pre-monitor
	// init phase.
	op sync.Mutex
}

func newController(config *viper.Viper) *controller {
	return &controller{
		config: config,
		graph:  graph.New[common.Service](),
		stats:  newStatsStore(),
	}
}

// register is wiring-time only: readers iterate c.services unlocked, so
// registering after Start races the monitor.
func (c *controller) register(service common.Service) {
	if c.stats.track(service) {
		// append late-registered services to the end of the sorted list
		if len(c.services) > 0 {
			c.services = append(c.services, service)
		}
	}
	c.graph.Add(service)
}

func (c *controller) addDependency(service, dep common.Service) {
	c.register(dep)
	c.graph.Add(service, dep)
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

func (c *controller) stop(service common.Service, wait bool) (bool, error) {
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
	err := svc.Stop(wait)
	c.stats.update(name, func(stat *entity.SupervisorStats) {
		stat.StopErr = err
		stat.StopDuration = time.Since(stoppedAt)
	})
	return true, err
}

// Init initializes one service under the operation lock - the manual
// InitService contract.
func (c *controller) Init(ctx context.Context, service common.Service) error {
	c.op.Lock()
	defer c.op.Unlock()
	_, err := c.init(ctx, service)
	return err
}

// Start starts one service under the operation lock - the manual
// StartService contract.
func (c *controller) Start(service common.Service) error {
	c.op.Lock()
	defer c.op.Unlock()
	_, err := c.start(service)
	return err
}

// Stop stops one service under the operation lock - the manual
// StopService contract.
func (c *controller) Stop(service common.Service, wait bool) error {
	c.op.Lock()
	defer c.op.Unlock()
	_, err := c.stop(service, wait)
	return err
}

// Restart forces a stop-init-start cycle under the operation lock
// regardless of state - the manual RestartService contract, which
// resurrects even a stopped service.
func (c *controller) Restart(ctx context.Context, service common.Service) error {
	c.op.Lock()
	defer c.op.Unlock()
	return c.restart(ctx, service)
}

// RestartIfRunning is the monitor's restart: under the operation lock it
// re-checks Stopped, so a deliberate stop that landed after the sweep's
// snapshot stays stopped instead of being resurrected.
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
		if err := svc.Stop(true); err != nil {
			c.log.Errorf("failed to stop service %s for restart: %s", name, err)
		}
		c.stats.update(name, func(stat *entity.SupervisorStats) {
			stat.Stopped = false
		})
	}
	// restarted stamps the attempt and carries the outcome into the
	// healthcheck verdict, so stats are coherent before the next sweep:
	// success clears it, failure records the phase error that broke the
	// cycle.
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

// initTurn runs one service's init turn. With an init policy set, the turn
// waits for dependencies to be init-ready, then runs the service's own
// Init, retrying transient blockages with exponential backoff until the
// policy or the context ends the turn. A dependency that already gave up
// its own turn is permanent: waiting cannot fix it, so the turn fails
// fast. ok reports whether the service's own Init ran.
func (c *controller) initTurn(ctx context.Context, service common.Service) (bool, error) {
	name := service.Name()
	// map the policy onto retry-go: 0 -> a single attempt, n -> n retries
	// after the first, -1 -> unlimited, bounded by ctx
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
		// the turn never reached its own Init - record why
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

// depsInitReady reports what blocks a service's init: nil when every
// dependency is initialized and - with an init policy set - answering its
// Ready probe, if it has one. transient tells whether waiting could clear
// the blockage: a failing probe is transient, a dependency that failed its
// own init turn is not. A one-pass init (no policy) never probes.
func (c *controller) depsInitReady(ctx context.Context, service common.Service) (blocked error, transient bool) {
	for _, dep := range service.Dependencies() {
		if dep == nil {
			panic(errors.Newf("%s dependency should not be nil, pls remove optional service from Dependencies()", service.Name()))
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

// StopAll stops every service under the operation lock, honoring the stop
// policy's timeout.
func (c *controller) StopAll(wait bool) error {
	c.op.Lock()
	defer c.op.Unlock()
	return c.stopAll(wait)
}

func (c *controller) stopAll(wait bool) error {
	if c.stopPolicy.Timeout > 0 {
		// on timeout the worker keeps stopping in the background and a hung
		// Stop leaks it - acceptable because the process is expected to
		// exit right after a timed-out shutdown
		done := make(chan error, 1)
		go func() {
			done <- c.stopAllServices(wait)
		}()
		select {
		case err := <-done:
			return err
		case <-time.After(c.stopPolicy.Timeout):
			c.log.Warnf("shutdown timed out after %s", c.stopPolicy.Timeout)
			return errors.DeadlineExceeded.Newf("shutdown timed out after %s", c.stopPolicy.Timeout)
		}
	}
	return c.stopAllServices(wait)
}

func (c *controller) stopAllServices(wait bool) error {
	c.log.Info("stopping services...")
	var errs []error
	var total, failed int
	l := len(c.services)
	// stop services in reversed order
	for i := l - 1; i > -1; i-- {
		service := c.services[i]
		ok, err := c.stop(service, wait)
		if ok {
			if err != nil {
				failed++
				errs = append(errs, errors.Wrapf(err, "service %s", service.Name()))
			}
			total++
		}
	}
	c.log.Infof("%d services stopped, %d failed", total, failed)
	return errors.Combine(errs...)
}
