package supervisor

import (
	"context"

	"github.com/xhanio/errors"
)

// Ready is nil iff every service is ready, judged from cached stats.
func (m *manager) Ready(_ context.Context) error {
	var errs []error
	for _, svc := range m.c.services {
		stat := m.c.stats.snapshot(svc.Name())
		if stat == nil || stat.Ready {
			continue
		}
		if stat.ReadinessErr != nil {
			errs = append(errs, errors.Wrapf(stat.ReadinessErr, "service %s not ready", svc.Name()))
		} else {
			errs = append(errs, errors.Unavailable.Newf("service %s not ready", svc.Name()))
		}
	}
	return errors.Combine(errs...)
}

// Alive fails only when a service's liveness fails with restarts exhausted (or
// maxRetries 0); unlimited retries never escalate.
func (m *manager) Alive(_ context.Context) error {
	maxRetries := m.monitor.policy.MaxRetries
	if maxRetries < 0 {
		return nil
	}
	var errs []error
	for _, svc := range m.c.services {
		stat := m.c.stats.snapshot(svc.Name())
		if stat == nil || stat.LivenessErr == nil {
			continue
		}
		if stat.Restarts >= maxRetries {
			errs = append(errs, errors.Wrapf(stat.LivenessErr,
				"service %s failed liveness with recovery spent (%d/%d restarts)",
				svc.Name(), stat.Restarts, maxRetries))
		}
	}
	return errors.Combine(errs...)
}
