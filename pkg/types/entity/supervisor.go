package entity

import (
	"time"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/types/common"
)

// SupervisorInitPolicy shapes how a service's init turn retries: MaxRetries bounds
// the attempts (0 one pass, -1 until ctx cancels), Delay is the backoff
// base, doubling up to MaxDelay.
type SupervisorInitPolicy struct {
	MaxRetries int
	Delay      time.Duration
	MaxDelay   time.Duration
}

// SupervisorMonitorPolicy shapes the health monitor: Interval is the sweep
// cadence (0 disables monitoring), MaxRetries bounds in-process restarts
// per service (0 none - a liveness failure escalates immediately, -1
// unlimited), RestartDelay pauses before each restart attempt.
type SupervisorMonitorPolicy struct {
	Interval     time.Duration
	MaxRetries   int
	RestartDelay time.Duration
}

// SupervisorStopPolicy shapes how a shutdown ends: Timeout bounds the whole stop,
// 0 waits indefinitely.
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
