package supervisor

import (
	"context"
	"time"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/log"
)

type monitor struct {
	log    log.Logger
	policy entity.SupervisorMonitorPolicy
	c      *controller
}

func (mon *monitor) run(ctx context.Context) {
	ticker := time.NewTicker(mon.policy.Interval)
	defer ticker.Stop()
	mon.log.Infof("health monitor started (interval: %s)", mon.policy.Interval)
	for {
		select {
		case <-ctx.Done():
			mon.log.Info("health monitor stopped")
			return
		case <-ticker.C:
			mon.checkAll(ctx)
		}
	}
}

func (mon *monitor) checkAll(ctx context.Context) {
	sw := newSweep()
	for _, service := range mon.c.services {
		select {
		case <-ctx.Done():
			return
		default:
		}
		stat := mon.c.stats.snapshot(service.Name())
		if stat == nil || stat.Stopped {
			continue
		}
		if err := mon.check(ctx, service, sw); err != nil {
			mon.log.Warnf("healthcheck failed for %s: %s", service.Name(), err)
		}
		// re-snapshot: check just wrote the probe results
		stat = mon.c.stats.snapshot(service.Name())
		// readiness-only failures don't restart
		if stat.LivenessErr == nil && stat.Healthcheck() == nil {
			continue
		}
		if mon.policy.MaxRetries != 0 {
			if mon.policy.RestartDelay > 0 {
				select {
				case <-ctx.Done():
					return
				case <-time.After(mon.policy.RestartDelay):
				}
			}
			select {
			case <-ctx.Done():
				return
			default:
			}
			if mon.policy.MaxRetries >= 0 && stat.Restarts >= mon.policy.MaxRetries {
				mon.log.Warnf("service %s reached max restart attempts (%d)", service.Name(), mon.policy.MaxRetries)
				continue
			}
			if err := mon.c.RestartIfRunning(ctx, service); err != nil {
				mon.log.Errorf("failed to restart service %s: %s", service.Name(), err)
			}
		}
	}
}

// sweep memoizes probes so a shared dependency is checked once per pass.
type sweep struct {
	done     map[string]error
	inflight map[string]bool
}

func newSweep() *sweep {
	return &sweep{
		done:     make(map[string]error),
		inflight: make(map[string]bool),
	}
}

func (mon *monitor) healthcheck(ctx context.Context, service common.Service) error {
	return mon.check(ctx, service, newSweep())
}

func (mon *monitor) check(ctx context.Context, service common.Service, sw *sweep) error {
	if service == nil {
		return nil
	}
	name := service.Name()
	if err, checked := sw.done[name]; checked {
		return err
	}
	if sw.inflight[name] {
		// cycle guard: TopoSort should have rejected it
		return nil
	}
	sw.inflight[name] = true
	defer delete(sw.inflight, name)
	var errs []error
	for _, dep := range service.Dependencies() {
		if dep == nil {
			continue // an optional dependency the app did not wire: nothing to probe
		}
		errs = append(errs, mon.check(ctx, dep, sw))
	}
	stat := mon.c.stats.snapshot(name)
	if stat == nil {
		err := errors.Combine(errs...)
		sw.done[name] = err
		return err
	}
	errs = append(errs, stat.Healthcheck())
	// probe unlocked: probes may do I/O
	var livenessErr error
	if liveness, ok := service.(common.Liveness); ok {
		if err := liveness.Alive(ctx); err != nil {
			livenessErr = err
			errs = append(errs, errors.Wrapf(err, "liveness %s", name))
		}
	}
	var readinessErr error
	readiness, probesReadiness := service.(common.Readiness)
	if probesReadiness {
		if err := readiness.Ready(ctx); err != nil {
			readinessErr = err
			errs = append(errs, errors.Wrapf(err, "readiness %s", name))
		}
	}
	healthcheckErr := errors.Combine(errs...)
	mon.c.stats.update(name, func(stat *entity.SupervisorStats) {
		stat.LivenessErr = livenessErr
		stat.ReadinessErr = readinessErr
		if probesReadiness {
			stat.Ready = readinessErr == nil
		}
		stat.HealthcheckedAt = time.Now()
		stat.HealthcheckErr = healthcheckErr
	})
	sw.done[name] = healthcheckErr
	return healthcheckErr
}
