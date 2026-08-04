package supervisor

import (
	"context"

	"github.com/xhanio/errors"
)

// Ready implements common.Readiness as the graph's roll-up: nil iff every
// registered service is ready, judged from the stats the monitor already
// maintains. Probing stays the monitor's job; readiness is the transient
// signal - stop routing traffic here, keep the process, let the monitor
// work the problem.
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

// Alive implements common.Liveness for the one failure the supervisor
// cannot recover from itself: a service whose liveness keeps failing after
// in-process recovery is spent - restarts exhausted, or restarts never
// configured (maxRetries 0), in which case the platform is the recovery
// path and escalation is immediate. Unlimited retries (maxRetries < 0)
// never escalate: the monitor keeps trying, so the process stays alive.
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
