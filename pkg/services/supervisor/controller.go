package supervisor

import (
	"context"
	"sync"
	"time"

	"github.com/spf13/viper"
	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/structs/graph"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/confutil"
	"github.com/xhanio/framingo/pkg/utils/log"
)

type controller struct {
	log             log.Logger
	config          *viper.Viper
	mu              sync.Mutex
	shutdownTimeout time.Duration
	graph           graph.Graph[common.Service]
	services        []common.Service
	stats           map[string]*entity.SupervisorStats
}

func newController(config *viper.Viper) *controller {
	return &controller{
		config: config,
		graph:  graph.New[common.Service](),
		stats:  make(map[string]*entity.SupervisorStats),
	}
}

func (c *controller) register(service common.Service) {
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

func (c *controller) stat(name string) *entity.SupervisorStats {
	return c.stats[name]
}

func (c *controller) init(ctx context.Context, service common.Service) (bool, error) {
	svc, ok := service.(common.Initializable)
	if !ok {
		if stat := c.stat(service.Name()); stat != nil {
			stat.Initialized = true
			stat.Ready = true
		}
		return false, nil
	}
	c.log.Debugf("initializing %s", service.Name())
	if c.config != nil {
		ctx = confutil.WrapContext(ctx, c.config)
	}
	stat := c.stat(service.Name())
	stat.InitializedAt = time.Now()
	err := svc.Init(ctx)
	stat.InitDuration = time.Since(stat.InitializedAt)
	stat.Initialized = err == nil
	stat.InitializationErr = err
	stat.Ready = err == nil
	return true, err
}

func (c *controller) start(service common.Service) (bool, error) {
	svc, ok := service.(common.Daemon)
	if !ok {
		return false, nil
	}
	c.log.Debugf("starting %s", service.Name())
	stat := c.stat(service.Name())
	stat.Started = true
	stat.Stopped = false
	stat.StartedAt = time.Now()
	stat.StartErr = svc.Start(context.Background())
	stat.StartDuration = time.Since(stat.StartedAt)
	stat.Ready = stat.StartErr == nil
	return true, stat.StartErr
}

func (c *controller) stop(service common.Service, wait bool) (bool, error) {
	svc, ok := service.(common.Daemon)
	if !ok {
		return false, nil
	}
	c.log.Debugf("stopping %s", service.Name())
	stat := c.stat(service.Name())
	stat.Stopped = true
	stat.Ready = false
	stat.StoppedAt = time.Now()
	stat.StopErr = svc.Stop(wait)
	stat.StopDuration = time.Since(stat.StoppedAt)
	return true, stat.StopErr
}

func (c *controller) restart(ctx context.Context, service common.Service) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	stat := c.stat(service.Name())
	c.log.Infof("restarting service %s (attempt %d)", service.Name(), stat.Restarts+1)
	if svc, ok := service.(common.Daemon); ok {
		if err := svc.Stop(true); err != nil {
			c.log.Errorf("failed to stop service %s for restart: %s", service.Name(), err)
		}
		stat.Stopped = false
	}
	if _, err := c.init(ctx, service); err != nil {
		stat.Restarts++
		stat.RestartedAt = time.Now()
		return err
	}
	if _, err := c.start(service); err != nil {
		stat.Restarts++
		stat.RestartedAt = time.Now()
		return err
	}
	stat.Restarts++
	stat.RestartedAt = time.Now()
	stat.HealthcheckErr = nil
	c.log.Infof("service %s restarted successfully", service.Name())
	return nil
}

func (c *controller) initAll(ctx context.Context) error {
	c.log.Info("initializing services...")
	var errs []error
	var total, failed int
	for _, service := range c.services {
		ready := true
		for _, dep := range service.Dependencies() {
			if dep == nil {
				panic(errors.Newf("%s dependency should not be nil, pls remove optional service from Dependencies()", service.Name()))
			}
			if stat := c.stat(dep.Name()); stat != nil && !stat.Initialized {
				c.log.Debugf("%s dependency %s is not initialized", service.Name(), dep.Name())
				ready = false
				break
			}
		}
		if !ready {
			stat := c.stat(service.Name())
			stat.InitializationErr = errors.Newf("dependencies not ready")
			errs = append(errs, errors.Wrapf(stat.InitializationErr, "service %s", service.Name()))
			failed++
			continue
		}
		ok, err := c.init(ctx, service)
		if ok {
			if err != nil {
				failed++
				errs = append(errs, errors.Wrapf(err, "service %s", service.Name()))
			}
			total++
		}
	}
	c.log.Infof("%d services initialized, %d failed", total, failed)
	return errors.Combine(errs...)
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
	if c.shutdownTimeout > 0 {
		done := make(chan error, 1)
		go func() {
			done <- c.stopAllServices(wait)
		}()
		select {
		case err := <-done:
			return err
		case <-time.After(c.shutdownTimeout):
			c.log.Warnf("shutdown timed out after %s", c.shutdownTimeout)
			return errors.DeadlineExceeded.Newf("shutdown timed out after %s", c.shutdownTimeout)
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
