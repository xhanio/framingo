package supervisor

import (
	"context"
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

// Stats returns a point-in-time copy of each service's stats, so callers
// read them without racing the monitor's ongoing sweeps. The order is
// topological - dependencies above dependents, the same order services
// init - so a red dependency explains the red services below it.
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

// The manual per-service operations go through the controller's exported
// methods, which serialize under its operation lock - they never interleave
// with a monitor restart's stop-init-start cycle at the service level.

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

func (m *manager) StopService(name string, wait bool) error {
	service := m.c.find(name)
	if service == nil {
		return errors.NotFound.Newf("service %s not found", name)
	}
	return m.c.Stop(service, wait)
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
