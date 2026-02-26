package app

import (
	"context"
	"sync"
	"time"

	"github.com/spf13/viper"
	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/structs/graph"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/confutil"
	"github.com/xhanio/framingo/pkg/utils/log"
)

type lifecycle struct {
	log             log.Logger
	config          *viper.Viper
	mu              sync.Mutex
	shutdownTimeout time.Duration
	graph           graph.Graph[common.Service]
	services        []common.Service
	stats           map[string]*Stats
}

func newLifecycle(config *viper.Viper) *lifecycle {
	return &lifecycle{
		config: config,
		graph:  graph.New[common.Service](),
		stats:  make(map[string]*Stats),
	}
}

func (lc *lifecycle) register(service common.Service) {
	if _, ok := lc.stats[service.Name()]; !ok {
		lc.stats[service.Name()] = &Stats{
			Name:   service.Name(),
			source: service,
		}
	}
	lc.graph.Add(service)
}

func (lc *lifecycle) addDependency(service, dep common.Service) {
	lc.register(dep)
	lc.graph.Add(service, dep)
}

func (lc *lifecycle) topoSort() error {
	err := lc.graph.TopoSort()
	if err != nil {
		return errors.Wrap(err)
	}
	lc.services = lc.graph.Nodes()
	return nil
}

func (lc *lifecycle) find(name string) common.Service {
	for _, service := range lc.services {
		if service.Name() == name {
			return service
		}
	}
	return nil
}

func (lc *lifecycle) stat(name string) *Stats {
	return lc.stats[name]
}

func (lc *lifecycle) init(ctx context.Context, service common.Service) (bool, error) {
	svc, ok := service.(common.Initializable)
	if !ok {
		return false, nil
	}
	lc.log.Debugf("initializing %s", service.Name())
	if lc.config != nil {
		ctx = confutil.WrapContext(ctx, lc.config)
	}
	stat := lc.stat(service.Name())
	stat.InitializedAt = time.Now()
	err := svc.Init(ctx)
	stat.InitDuration = time.Since(stat.InitializedAt)
	stat.Initialized = err == nil
	stat.InitializationErr = err
	stat.Ready = err == nil
	return true, err
}

func (lc *lifecycle) start(service common.Service) (bool, error) {
	svc, ok := service.(common.Daemon)
	if !ok {
		return false, nil
	}
	lc.log.Debugf("starting %s", service.Name())
	stat := lc.stat(service.Name())
	stat.Started = true
	stat.Stopped = false
	stat.StartedAt = time.Now()
	stat.StartErr = svc.Start(context.Background())
	stat.StartDuration = time.Since(stat.StartedAt)
	stat.Ready = stat.StartErr == nil
	return true, stat.StartErr
}

func (lc *lifecycle) stop(service common.Service, wait bool) (bool, error) {
	svc, ok := service.(common.Daemon)
	if !ok {
		return false, nil
	}
	lc.log.Debugf("stopping %s", service.Name())
	stat := lc.stat(service.Name())
	stat.Stopped = true
	stat.Ready = false
	stat.StoppedAt = time.Now()
	stat.StopErr = svc.Stop(wait)
	stat.StopDuration = time.Since(stat.StoppedAt)
	return true, stat.StopErr
}

func (lc *lifecycle) restart(ctx context.Context, service common.Service) error {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	stat := lc.stat(service.Name())
	lc.log.Infof("restarting service %s (attempt %d)", service.Name(), stat.Restarts+1)
	if svc, ok := service.(common.Daemon); ok {
		if err := svc.Stop(true); err != nil {
			lc.log.Errorf("failed to stop service %s for restart: %s", service.Name(), err)
		}
		stat.Stopped = false
	}
	if _, err := lc.init(ctx, service); err != nil {
		stat.Restarts++
		stat.RestartedAt = time.Now()
		return err
	}
	if _, err := lc.start(service); err != nil {
		stat.Restarts++
		stat.RestartedAt = time.Now()
		return err
	}
	stat.Restarts++
	stat.RestartedAt = time.Now()
	stat.HealthcheckErr = nil
	lc.log.Infof("service %s restarted successfully", service.Name())
	return nil
}

func (lc *lifecycle) initAll(ctx context.Context) error {
	lc.log.Info("initializing services...")
	var errs []error
	var total, failed int
	for _, service := range lc.services {
		ready := true
		for _, dep := range service.Dependencies() {
			if dep == nil {
				panic(errors.Newf("%s dependency should not be nil, pls remove optional service from Dependencies()", service.Name()))
			}
			if stat := lc.stat(dep.Name()); stat != nil && !stat.Initialized {
				lc.log.Debugf("%s dependency %s is not initialized", service.Name(), dep.Name())
				ready = false
				break
			}
		}
		if !ready {
			stat := lc.stat(service.Name())
			stat.InitializationErr = errors.Newf("dependencies not ready")
			errs = append(errs, errors.Wrapf(stat.InitializationErr, "service %s", service.Name()))
			failed++
			continue
		}
		ok, err := lc.init(ctx, service)
		if ok {
			if err != nil {
				failed++
				errs = append(errs, errors.Wrapf(err, "service %s", service.Name()))
			}
			total++
		}
	}
	lc.log.Infof("%d services initialized, %d failed", total, failed)
	return errors.Combine(errs...)
}

func (lc *lifecycle) startAll() error {
	lc.log.Info("starting services...")
	var errs []error
	var total, failed int
	for _, service := range lc.services {
		ok, err := lc.start(service)
		if ok {
			if err != nil {
				failed++
				errs = append(errs, errors.Wrapf(err, "service %s", service.Name()))
			}
			total++
		}
	}
	lc.log.Infof("%d services started, %d failed", total, failed)
	return errors.Combine(errs...)
}

func (lc *lifecycle) stopAll(wait bool) error {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	if lc.shutdownTimeout > 0 {
		done := make(chan error, 1)
		go func() {
			done <- lc.stopAllServices(wait)
		}()
		select {
		case err := <-done:
			return err
		case <-time.After(lc.shutdownTimeout):
			lc.log.Warnf("shutdown timed out after %s", lc.shutdownTimeout)
			return errors.DeadlineExceeded.Newf("shutdown timed out after %s", lc.shutdownTimeout)
		}
	}
	return lc.stopAllServices(wait)
}

func (lc *lifecycle) stopAllServices(wait bool) error {
	lc.log.Info("stopping services...")
	var errs []error
	var total, failed int
	l := len(lc.services)
	// stop services in reversed order
	for i := l - 1; i > -1; i-- {
		service := lc.services[i]
		ok, err := lc.stop(service, wait)
		if ok {
			if err != nil {
				failed++
				errs = append(errs, errors.Wrapf(err, "service %s", service.Name()))
			}
			total++
		}
	}
	lc.log.Infof("%d services stopped, %d failed", total, failed)
	return errors.Combine(errs...)
}
