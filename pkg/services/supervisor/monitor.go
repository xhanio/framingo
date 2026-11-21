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

// checkAll probes every service not stopped, and restarts, per the policy,
// each that fails on its own.
func (mon *monitor) checkAll(ctx context.Context) {
	sw := newSweep()
	for _, service := range mon.c.services {
		if ctx.Err() != nil {
			return
		}
		if stat := mon.c.stats.snapshot(service.Name()); stat == nil || stat.Stopped {
			continue
		}
		if err := mon.check(ctx, service, sw); err != nil {
			mon.log.Warnf("healthcheck failed for %s: %s", service.Name(), err)
		}
		if mon.failing(service) && !mon.restart(ctx, service) {
			return
		}
	}
}

// failing: its liveness, or its own Init, Start or Reconcile, has failed, as
// check just recorded. Readiness alone never is, nor a dependency's failure.
func (mon *monitor) failing(service common.Service) bool {
	stat := mon.c.stats.snapshot(service.Name())
	return stat != nil && (stat.LivenessErr != nil || stat.Healthcheck() != nil)
}

// restart restarts a failing service per the policy, after its delay: never
// with maxRetries 0, where the platform is the recovery, and no more once its
// restarts are spent. It returns false once ctx ends.
func (mon *monitor) restart(ctx context.Context, service common.Service) bool {
	if mon.policy.MaxRetries == 0 {
		return true
	}
	stat := mon.c.stats.snapshot(service.Name())
	if mon.policy.MaxRetries > 0 && stat != nil && stat.Restarts >= mon.policy.MaxRetries {
		mon.log.Warnf("service %s reached max restart attempts (%d)", service.Name(), mon.policy.MaxRetries)
		return true
	}
	if !sleep(ctx, mon.policy.RestartDelay) {
		return false
	}
	if err := mon.c.RestartIfRunning(ctx, service); err != nil {
		mon.log.Errorf("failed to restart service %s: %s", service.Name(), err)
	}
	return true
}

// sleep waits d, and reports false if ctx ends first.
func sleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
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
