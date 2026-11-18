package server

import (
	"github.com/xhanio/errors"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/semconv/v1.43.0/httpconv"
)

// built once at Add and reused across restarts: the provider is stable for the process
type instruments struct {
	duration     httpconv.ServerRequestDuration  // http.server.request.duration, s
	active       httpconv.ServerActiveRequests   // http.server.active_requests
	requestSize  httpconv.ServerRequestBodySize  // http.server.request.body.size, By
	responseSize httpconv.ServerResponseBodySize // http.server.response.body.size, By
}

// scoped by server name so listeners sharing a meter stay separate; no meter means noop instruments
func (s *server) newInstruments() (*instruments, error) {
	provider := metric.MeterProvider(noop.NewMeterProvider())
	if s.meter != nil {
		provider = s.meter.Provider()
	}
	m := provider.Meter(s.name)
	var inst instruments
	var err error
	if inst.duration, err = httpconv.NewServerRequestDuration(m); err != nil {
		return nil, errors.Wrap(err)
	}
	if inst.active, err = httpconv.NewServerActiveRequests(m); err != nil {
		return nil, errors.Wrap(err)
	}
	if inst.requestSize, err = httpconv.NewServerRequestBodySize(m); err != nil {
		return nil, errors.Wrap(err)
	}
	if inst.responseSize, err = httpconv.NewServerResponseBodySize(m); err != nil {
		return nil, errors.Wrap(err)
	}
	return &inst, nil
}
