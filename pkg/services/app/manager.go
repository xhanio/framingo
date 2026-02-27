package app

import (
	"context"
	"io"
	"path"
	"sort"
	"sync"
	"time"

	"github.com/spf13/viper"
	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/printutil"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"
)

type manager struct {
	name string
	log  log.Logger

	cancel context.CancelFunc
	wg     *sync.WaitGroup

	lc      *lifecycle
	monitor *monitor
}

func New(config *viper.Viper, opts ...Option) Manager {
	return newManager(config, opts...)
}

func newManager(config *viper.Viper, opts ...Option) *manager {
	lc := newLifecycle(config)
	m := &manager{
		log: log.Default,
		wg:  &sync.WaitGroup{},
		lc:  lc,
		monitor: &monitor{
			lc: lc,
		},
	}
	m.apply(opts...)
	m.log = m.log.By(m)
	m.lc.log = m.log
	m.monitor.log = m.log
	return m
}

func (m *manager) Name() string {
	if m.name == "" {
		m.name = path.Join(reflectutil.Locate(m))
	}
	return m.name
}

func (m *manager) Dependencies() []common.Service {
	return nil
}

func (m *manager) Register(services ...common.Service) {
	for _, service := range services {
		if service == nil {
			continue
		}
		m.lc.register(service)
		for _, dep := range service.Dependencies() {
			if dep != nil {
				m.lc.addDependency(service, dep)
			}
		}
	}
}

func (m *manager) TopoSort() error {
	now := time.Now()
	if err := m.lc.topoSort(); err != nil {
		return err
	}
	m.log.Debugf("%d services sorted in %s", len(m.lc.services), time.Since(now))
	return nil
}

func (m *manager) Services() []common.Service {
	return m.lc.services
}

func (m *manager) Stats() ([]*Stats, error) {
	var result []*Stats
	sorted := make([]common.Service, len(m.lc.services))
	copy(sorted, m.lc.services)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name() > sorted[j].Name()
	})
	for _, svc := range sorted {
		result = append(result, m.lc.stat(svc.Name()))
	}
	var errs []error
	for _, stat := range result {
		errs = append(errs, stat.Healthcheck())
	}
	return result, errors.Combine(errs...)
}

func (m *manager) Init(ctx context.Context) error {
	return m.lc.initAll(ctx)
}

func (m *manager) InitService(ctx context.Context, name string) error {
	service := m.lc.find(name)
	if service == nil {
		return errors.NotFound.Newf("service %s not found", name)
	}
	_, err := m.lc.init(ctx, service)
	return err
}

func (m *manager) StartService(name string) error {
	service := m.lc.find(name)
	if service == nil {
		return errors.NotFound.Newf("service %s not found", name)
	}
	_, err := m.lc.start(service)
	return err
}

func (m *manager) StopService(name string, wait bool) error {
	service := m.lc.find(name)
	if service == nil {
		return errors.NotFound.Newf("service %s not found", name)
	}
	_, err := m.lc.stop(service, wait)
	return err
}

func (m *manager) RestartService(ctx context.Context, name string) error {
	service := m.lc.find(name)
	if service == nil {
		return errors.NotFound.Newf("service %s not found", name)
	}
	return m.lc.restart(ctx, service)
}

func (m *manager) Migrate() error {
	return errors.NotImplemented
}

func (m *manager) Start(ctx context.Context) error {
	if m.cancel != nil {
		m.log.Warnf("%s already started", m.Name())
		return nil
	}
	if err := m.lc.startAll(); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		<-ctx.Done()
		m.log.Infof("service %s stopped", m.Name())
	}()
	if m.monitor.interval > 0 {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			m.monitor.run(ctx)
		}()
	}
	return nil
}

func (m *manager) Stop(wait bool) error {
	if m.cancel == nil {
		m.log.Warnf("%s already stopped", m.Name())
		return nil
	}
	m.cancel()
	if wait {
		m.wg.Wait()
	}
	m.cancel = nil
	return m.lc.stopAll(wait)
}

func (m *manager) Info(w io.Writer, debug bool) {
	t := printutil.NewTable(w)
	t.Header("service status")
	t.Title("service", "alive", "ready", "uptime", "init_err", "start_err", "healthcheck_err")
	stats, _ := m.Stats() // errors are displayed in the table below
	for _, stat := range stats {
		_ = m.monitor.healthcheck(stat.source) // refreshes stat fields for display
		alive := stat.LivenessErr == nil && stat.Healthcheck() == nil
		t.Row(stat.Name, alive, stat.Ready, stat.Uptime(), stat.InitializationErr, stat.StartErr, stat.HealthcheckErr)
	}
	t.NewLine()
	t.Flush()
	for _, service := range m.lc.services {
		if svc, ok := service.(common.Debuggable); ok {
			svc.Info(w, debug)
		}
	}
}
