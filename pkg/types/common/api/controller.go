package api

import (
	"time"
)

type SystemReadiness struct {
	HasSN    bool           `json:"has_sn"`
	Services []*ServiceStat `json:"services,omitempty"`
}

type ServiceStat struct {
	Name              string    `json:"name"`
	Initialized       bool      `json:"initialized"`
	InitializedAt     time.Time `json:"initialized_at"`
	InitializationErr string    `json:"initialization_err"`
	Started           bool      `json:"started"`
	StartedAt         time.Time `json:"started_at"`
	StartErr          string    `json:"start_err"`
	Stopped           bool      `json:"stopped"`
	StoppedAt         time.Time `json:"stopped_at"`
	StopErr           string    `json:"stop_err"`
	HealthcheckedAt   time.Time `json:"healthchecked_at"`
	HealthcheckErr    string    `json:"healthcheck_err"`
}
