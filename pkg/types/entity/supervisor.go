package entity

import (
	"time"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/types/common"
)

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
