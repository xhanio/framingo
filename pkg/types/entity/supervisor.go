package entity

import (
	"time"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/types/common"
)

// SupervisorInitPolicy: MaxRetries 0 is one pass, -1 retries until ctx ends;
// Delay doubles up to MaxDelay.
type SupervisorInitPolicy struct {
	MaxRetries int
	Delay      time.Duration
	MaxDelay   time.Duration
}

// SupervisorMonitorPolicy: Interval 0 disables monitoring; MaxRetries bounds
// restarts per service (0 escalates a liveness failure at once, -1 unlimited).
type SupervisorMonitorPolicy struct {
	Interval     time.Duration
	MaxRetries   int
	RestartDelay time.Duration
}

// SupervisorStopPolicy: Timeout bounds each full stop and each restart's stop,
// then forces the rest; 0 defers to the caller's ctx.
type SupervisorStopPolicy struct {
	Timeout time.Duration
}

type SupervisorStats struct {
	Name              string
	Initialized       bool
	InitializedAt     time.Time
	InitializationErr error
	Started           bool
	StartedAt         time.Time
	StartErr          error
	Stopped           bool
	StoppedAt         time.Time
	StopErr           error
	HealthcheckedAt   time.Time
	HealthcheckErr    error
	LivenessErr       error
	Ready             bool
	ReadinessErr      error
	Restarts          int
	RestartedAt       time.Time
	InitDuration      time.Duration
	StartDuration     time.Duration
	StopDuration      time.Duration
	Source            common.Service
}

func (s *SupervisorStats) Uptime() time.Duration {
	if !s.Started || s.Stopped {
		return 0
	}
	return time.Since(s.StartedAt)
}

func (s *SupervisorStats) Healthcheck() error {
	var errs []error
	if s.Stopped {
		errs = append(errs, errors.Unavailable.Newf("service %s stopped", s.Name))
	}
	if s.InitializationErr != nil {
		errs = append(errs, errors.Wrapf(s.InitializationErr, "service %s", s.Name))
	}
	if s.StartErr != nil {
		errs = append(errs, errors.Wrapf(s.StartErr, "service %s", s.Name))
	}
	return errors.Combine(errs...)
}
