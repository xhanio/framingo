package supervisor

import (
	"context"
	"time"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
)

func (m *manager) Register(services ...common.Service) error {
	for _, service := range services {
		if service == nil {
			continue
		}
		if err := m.c.register(service); err != nil {
			return errors.Wrap(err)
		}
		for _, dep := range service.Dependencies() {
			if dep == nil {
				continue
			}
			if err := m.c.addDependency(service, dep); err != nil {
				return errors.Wrap(err)
			}
		}
	}
	return nil
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

// Stats returns copies, in topological order (dependencies first).
func (m *manager) Stats() ([]*entity.SupervisorStats, error) {
	var result []*entity.SupervisorStats
	var errs []error
	for _, svc := range m.c.services {
		stat := m.c.stats.snapshot(svc.Name())
		result = append(result, stat)
		errs = append(errs, stat.Healthcheck())
	}
	return result, errors.Combine(errs...)
}

// manual ops serialize with monitor restarts under the controller's operation lock

func (m *manager) InitService(ctx context.Context, name string) error {
	service := m.c.find(name)
	if service == nil {
		return errors.NotFound.Newf("service %s not found", name)
	}
	return m.c.Init(ctx, service)
}

func (m *manager) StartService(name string) error {
	service := m.c.find(name)
	if service == nil {
		return errors.NotFound.Newf("service %s not found", name)
	}
	return m.c.Start(service)
}

func (m *manager) StopService(ctx context.Context, name string) error {
	service := m.c.find(name)
	if service == nil {
		return errors.NotFound.Newf("service %s not found", name)
	}
	return m.c.Stop(ctx, service)
}

func (m *manager) RestartService(ctx context.Context, name string) error {
	service := m.c.find(name)
	if service == nil {
		return errors.NotFound.Newf("service %s not found", name)
	}
	return m.c.Restart(ctx, service)
}

func (m *manager) Migrate() error {
	return errors.NotImplemented
}
