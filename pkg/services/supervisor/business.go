package supervisor

import (
	"context"
	"sort"
	"time"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
)

func (m *manager) Register(services ...common.Service) {
	for _, service := range services {
		if service == nil {
			continue
		}
		m.c.register(service)
		for _, dep := range service.Dependencies() {
			if dep != nil {
				m.c.addDependency(service, dep)
			}
		}
	}
}

func (m *manager) TopoSort() error {
	now := time.Now()
	if err := m.c.topoSort(); err != nil {
		return err
	}
	m.log.Debugf("%d services sorted in %s", len(m.c.services), time.Since(now))
	return nil
}

func (m *manager) Services() []common.Service {
	return m.c.services
}

func (m *manager) Stats() ([]*entity.SupervisorStats, error) {
	var result []*entity.SupervisorStats
	sorted := make([]common.Service, len(m.c.services))
	copy(sorted, m.c.services)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name() > sorted[j].Name()
	})
	for _, svc := range sorted {
		result = append(result, m.c.stat(svc.Name()))
	}
	var errs []error
	for _, stat := range result {
		errs = append(errs, stat.Healthcheck())
	}
	return result, errors.Combine(errs...)
}

func (m *manager) InitService(ctx context.Context, name string) error {
	service := m.c.find(name)
	if service == nil {
		return errors.NotFound.Newf("service %s not found", name)
	}
	_, err := m.c.init(ctx, service)
	return err
}

func (m *manager) StartService(name string) error {
	service := m.c.find(name)
	if service == nil {
		return errors.NotFound.Newf("service %s not found", name)
	}
	_, err := m.c.start(service)
	return err
}

func (m *manager) StopService(name string, wait bool) error {
	service := m.c.find(name)
	if service == nil {
		return errors.NotFound.Newf("service %s not found", name)
	}
	_, err := m.c.stop(service, wait)
	return err
}

func (m *manager) RestartService(ctx context.Context, name string) error {
	service := m.c.find(name)
	if service == nil {
		return errors.NotFound.Newf("service %s not found", name)
	}
	return m.c.restart(ctx, service)
}

func (m *manager) Migrate() error {
	return errors.NotImplemented
}
