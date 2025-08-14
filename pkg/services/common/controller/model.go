package controller

import (
	"time"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/errors"
)

type Manager interface {
	common.Service
	common.Initializable
	common.Daemon
	common.Debuggable
	Register(services ...common.Service)
	TopoSort() error
	Services() []common.Service
	Stats() ([]*Stats, error)
	// Migrate() error
	// InitService(name string) error
	// StartService(name string) error
	// StopService(name string, wait bool) error
	// RestartService(name string) error
}

type Stats struct {
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
	source            common.Service
}

func (s *Stats) Healthcheck() error {
	var errs []error
	if s.Stopped {
		// service is stopped
		errs = append(errs, errors.Newf("service %s stopped", s.Name))
	}
	if s.Initialized && s.InitializationErr != nil {
		errs = append(errs, errors.Newf("failed to initialize service %s: %s", s.Name, s.InitializationErr))
	}
	if s.Started && s.StartErr != nil {
		errs = append(errs, errors.Newf("failed to start service %s: %s", s.Name, s.StartErr))
	}
	return errors.Combine(errs...)
}
