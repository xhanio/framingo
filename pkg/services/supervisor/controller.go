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
	mu         sync.Mutex
	initPolicy entity.SupervisorInitPolicy
	stopPolicy entity.SupervisorStopPolicy
	graph      graph.Graph[common.Service]
	services   []common.Service
	// statMu guards the stats map and every field of the records it holds:
	// the monitor goroutine writes probe results while health endpoints
	// read them. All access goes through snapshot and update.
	statMu sync.RWMutex
	stats  map[string]*entity.SupervisorStats
}

func newController(config *viper.Viper) *controller {
	return &controller{
		config: config,
		graph:  graph.New[common.Service](),
		stats:  make(map[string]*entity.SupervisorStats),
	}
}

func (c *controller) register(service common.Service) {
	c.statMu.Lock()
	if _, ok := c.stats[service.Name()]; !ok {
		c.stats[service.Name()] = &entity.SupervisorStats{
			Name:   service.Name(),
			Source: service,
		}
		// append late-registered services to the end of the sorted list
		if len(c.services) > 0 {
			c.services = append(c.services, service)
		}
	}
	c.statMu.Unlock()
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

// snapshot returns a copy of the named service's stats, or nil if the
// service is unknown. Copies are what leave the lock: callers read them
// freely while the monitor keeps writing the live record.
func (c *controller) snapshot(name string) *entity.SupervisorStats {
	c.statMu.RLock()
	defer c.statMu.RUnlock()
	stat, ok := c.stats[name]
	if !ok {
		return nil
	}
	cp := *stat
	return &cp
}

// update mutates the named service's stats under the write lock. Blocking
// work (probes, Init/Start/Stop) stays outside fn.
func (c *controller) update(name string, fn func(stat *entity.SupervisorStats)) {
	c.statMu.Lock()
	defer c.statMu.Unlock()
	if stat, ok := c.stats[name]; ok {
		fn(stat)
	}
}

func (c *controller) init(ctx context.Context, service common.Service) (bool, error) {
	name := service.Name()
	svc, ok := service.(common.Initializable)
	if !ok {
		c.update(name, func(stat *entity.SupervisorStats) {
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
	c.update(name, func(stat *entity.SupervisorStats) {
		stat.InitializedAt = initializedAt
	})
	err := svc.Init(ctx)
	c.update(name, func(stat *entity.SupervisorStats) {
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
	c.update(name, func(stat *entity.SupervisorStats) {
		stat.Started = true
		stat.Stopped = false
		stat.StartedAt = startedAt
	})
	err := svc.Start(context.Background())
	c.update(name, func(stat *entity.SupervisorStats) {
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
	c.update(name, func(stat *entity.SupervisorStats) {
		stat.Stopped = true
		stat.Ready = false
		stat.StoppedAt = stoppedAt
	})
	err := svc.Stop(wait)
	c.update(name, func(stat *entity.SupervisorStats) {
		stat.StopErr = err
		stat.StopDuration = time.Since(stoppedAt)
	})
	return true, err
}

func (c *controller) restart(ctx context.Context, service common.Service) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	name := service.Name()
	if stat := c.snapshot(name); stat != nil {
		c.log.Infof("restarting service %s (attempt %d)", name, stat.Restarts+1)
	}
	if svc, ok := service.(common.Daemon); ok {
		if err := svc.Stop(true); err != nil {
			c.log.Errorf("failed to stop service %s for restart: %s", name, err)
		}
		c.update(name, func(stat *entity.SupervisorStats) {
			stat.Stopped = false
		})
	}
	restarted := func(healthy bool) {
		c.update(name, func(stat *entity.SupervisorStats) {
			stat.Restarts++
			stat.RestartedAt = time.Now()
			if healthy {
				stat.HealthcheckErr = nil
			}
		})
	}
	if _, err := c.init(ctx, service); err != nil {
		restarted(false)
		return err
	}
	if _, err := c.start(service); err != nil {
		restarted(false)
		return err
	}
	restarted(true)
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
		c.update(name, func(stat *entity.SupervisorStats) {
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
		if stat := c.snapshot(dep.Name()); stat != nil && !stat.Initialized {
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

func (c *controller) stopAll(wait bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopPolicy.Timeout > 0 {
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
