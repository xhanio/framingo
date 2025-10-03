package controller

import (
	"context"
	"io"
	"path"
	"sort"
	"sync"
	"time"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/structs/graph"
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

	sg graph.Graph[common.Service]

	stats map[string]*Stats
}

func New(opts ...Option) Manager {
	return newService(opts...)
}

func newService(opts ...Option) *manager {
	m := &manager{
		wg:    &sync.WaitGroup{},
		sg:    graph.New[common.Service](),
		stats: make(map[string]*Stats),
	}
	for _, opt := range opts {
		opt(m)
	}
	if m.log == nil {
		m.log = log.Default
	}
	m.log = m.log.By(m)
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

func (m *manager) register(service common.Service) {
	if _, ok := m.stats[service.Name()]; !ok {
		m.stats[service.Name()] = &Stats{
			Name:   service.Name(),
			source: service,
		}
	}
}

func (m *manager) Register(services ...common.Service) {
	for _, service := range services {
		if service != nil {
			m.register(service)
			m.sg.Add(service)
		}
		for _, dep := range service.Dependencies() {
			if dep != nil {
				m.register(dep)
				m.sg.Add(service, dep)
			}
		}
	}
}

func (m *manager) TopoSort() error {
	now := time.Now()
	// for _, svc := range m.sg.Nodes() {
	// 	m.log.Warnf("before: %s", svc.Name())
	// }
	err := m.sg.TopoSort()
	if err != nil {
		return errors.Wrap(err)
	}
	// for _, svc := range m.sg.Nodes() {
	// 	m.log.Errorf("after: %s", svc.Name())
	// }
	m.log.Debugf("%d services sorted in %s", m.sg.Count(), time.Since(now))
	return nil
}

func (m *manager) Services() []common.Service {
	return m.sg.Nodes()
}

func (m *manager) Stats() ([]*Stats, error) {
	var result []*Stats
	sorted := make([]common.Service, m.sg.Count())
	copy(sorted, m.sg.Nodes())
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name() > sorted[j].Name()
	})
	for _, svc := range sorted {
		result = append(result, m.stats[svc.Name()])
	}
	var errs []error
	for _, stat := range result {
		errs = append(errs, stat.Healthcheck())
	}
	return result, errors.Combine(errs...)
}

func (m *manager) Init() error {
	if err := m.initAll(); err != nil {
		return errors.Wrap(err)
	}

	return nil
}

func (m *manager) Migrate() error {
	return errors.NotImplemented
}

func (m *manager) Start(ctx context.Context) error {
	if m.cancel != nil {
		m.log.Warnf("%s already started", m.Name())
		return nil
	}
	if err := m.startAll(); err != nil {
		return errors.Wrap(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		for {
			select {
			case <-ctx.Done():
				m.log.Infof("service %s stopped", m.Name())
				return
			}
		}
	}()
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
	if err := m.stopAll(wait); err != nil {
		return errors.Wrap(err)
	}
	return nil
}

func (m *manager) Info(w io.Writer, debug bool) {
	t := printutil.NewTable(w)
	t.Header("service status")
	t.Title("service", "ready", "init_err", "start_err", "healthcheck_err")
	stats, _ := m.Stats()
	for _, stat := range stats {
		healthcheck := m.healthcheck(stat.source)
		t.Row(stat.Name, healthcheck == nil, stat.InitializationErr, stat.StartErr, healthcheck)
	}
	t.NewLine()
	t.Flush()
	m.infoAll(w, debug)
}

func (m *manager) init(service common.Service) (bool, error) {
	stat := m.stats[service.Name()]
	stat.Initialized = true
	svc, ok := service.(common.Initializable)
	if ok {
		m.log.Debugf("initializing %s", service.Name())
		stat.InitializedAt = time.Now()
		err := svc.Init()
		if err != nil {
			stat.InitializationErr = err
			stat.Initialized = false
		}
	}
	return ok, stat.InitializationErr
}

func (m *manager) initAll() error {
	m.log.Info("initializing services...")
	var errs []error
	var total, failed int
	for _, service := range m.sg.Nodes() {
		initializable := true
		for _, dep := range service.Dependencies() {
			if dep == nil {
				panic(errors.Newf("%s dependency should not be nil, pls remove optional service from Dependencies()", service.Name()))
			}
			if stat, ok := m.stats[dep.Name()]; ok && !stat.Initialized {
				m.log.Debugf("%s dependency %s is not initialized", service.Name(), dep.Name())
				initializable = false
				break
			}
		}
		if initializable {
			ok, err := m.init(service)
			if ok {
				if err != nil {
					failed++
					errs = append(errs, errors.Wrapf(err, "service %s", service.Name()))
				}
				total++
			}
		} else {
			errs = append(errs, errors.Newf("failed to initialized %s: dependencies not ready", service.Name()))
		}
	}
	m.log.Infof("%d services initialized, %d failed", total, failed)
	return errors.Combine(errs...)
}

func (m *manager) start(service common.Service) (bool, error) {
	stat := m.stats[service.Name()]
	svc, ok := service.(common.Daemon)
	if ok {
		m.log.Debugf("starting %s", service.Name())
		stat.Started = true
		stat.StartedAt = time.Now()
		stat.StartErr = svc.Start(context.Background())
	}
	return ok, stat.StartErr
}

func (m *manager) startAll() error {
	m.log.Info("starting services...")
	var errs []error
	var total, failed int
	for _, service := range m.sg.Nodes() {
		ok, err := m.start(service)
		if ok {
			if err != nil {
				failed++
				errs = append(errs, errors.Wrapf(err, "service %s", service.Name()))
			}
			total++
		}
	}
	m.log.Infof("%d services started, %d failed", total, failed)
	return errors.Combine(errs...)
}

func (m *manager) stop(service common.Service, wait bool) (bool, error) {
	stat := m.stats[service.Name()]
	svc, ok := service.(common.Daemon)
	if ok {
		m.log.Debugf("stopping %s", service.Name())
		stat.Stopped = true
		stat.StoppedAt = time.Now()
		stat.StopErr = svc.Stop(wait)
	}
	return ok, stat.StopErr
}

func (m *manager) stopAll(wait bool) error {
	m.log.Info("stopping services...")
	var errs []error
	var total, failed int
	l := m.sg.Count()
	// stop services in reversed order
	for i := l - 1; i > -1; i-- {
		service := m.sg.Nodes()[i]
		ok, err := m.stop(service, wait)
		if ok {
			if err != nil {
				failed++
				errs = append(errs, errors.Wrapf(err, "service %s", service.Name()))
			}
			total++
		}
	}
	if wait {
		// TODO: wait for resources to be released
	}
	m.log.Infof("%d services stopped, %d failed", total, failed)
	return errors.Combine(errs...)
}

func (m *manager) healthcheck(service common.Service) error {
	if service == nil {
		return nil
	}
	var errs []error
	for _, dep := range service.Dependencies() {
		errs = append(errs, m.healthcheck(dep))
	}
	stat := m.stats[service.Name()]
	errs = append(errs, stat.Healthcheck())
	return errors.Combine(errs...)
}

func (m *manager) infoAll(w io.Writer, debug bool) {
	for _, service := range m.sg.Nodes() {
		if svc, ok := service.(common.Debuggable); ok {
			svc.Info(w, debug)
		}
	}
}
