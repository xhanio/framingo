package app

import (
	"context"
	"time"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"
)

type monitor struct {
	log          log.Logger
	interval     time.Duration
	maxRetries   int
	restartDelay time.Duration
	lc           *lifecycle
}

func (mon *monitor) run(ctx context.Context) {
	ticker := time.NewTicker(mon.interval)
	defer ticker.Stop()
	mon.log.Infof("health monitor started (interval: %s)", mon.interval)
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
	for _, service := range mon.lc.services {
		select {
		case <-ctx.Done():
			return
		default:
		}
		stat := mon.lc.stat(service.Name())
		if stat.Stopped {
			continue
		}
		if err := mon.healthcheck(service); err != nil {
			mon.log.Warnf("healthcheck failed for %s: %s", service.Name(), err)
		}
		// only restart on liveness or stat-based failures, not readiness-only
		if stat.LivenessErr == nil && stat.Healthcheck() == nil {
			continue
		}
		if mon.maxRetries != 0 {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if mon.restartDelay > 0 {
				time.Sleep(mon.restartDelay)
			}
			if mon.maxRetries >= 0 && stat.Restarts >= mon.maxRetries {
				mon.log.Warnf("service %s reached max restart attempts (%d)", service.Name(), mon.maxRetries)
				continue
			}
			if err := mon.lc.restart(ctx, service); err != nil {
				mon.log.Errorf("failed to restart service %s: %s", service.Name(), err)
			}
		}
	}
}

func (mon *monitor) healthcheck(service common.Service) error {
	if service == nil {
		return nil
	}
	var errs []error
	for _, dep := range service.Dependencies() {
		errs = append(errs, mon.healthcheck(dep))
	}
	stat := mon.lc.stat(service.Name())
	if stat == nil {
		return errors.Combine(errs...)
	}
	errs = append(errs, stat.Healthcheck())
	stat.LivenessErr = nil
	if liveness, ok := service.(common.Liveness); ok {
		if err := liveness.Alive(); err != nil {
			stat.LivenessErr = err
			errs = append(errs, errors.Wrapf(err, "liveness %s", service.Name()))
		}
	}
	stat.ReadinessErr = nil
	if readiness, ok := service.(common.Readiness); ok {
		if err := readiness.Ready(); err != nil {
			stat.ReadinessErr = err
			stat.Ready = false
			errs = append(errs, errors.Wrapf(err, "readiness %s", service.Name()))
		} else {
			stat.Ready = true
		}
	}
	stat.HealthcheckedAt = time.Now()
	stat.HealthcheckErr = errors.Combine(errs...)
	return stat.HealthcheckErr
}
