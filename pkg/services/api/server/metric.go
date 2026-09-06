package server

import (
	"net/http"

	"github.com/xhanio/errors"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/semconv/v1.43.0/httpconv"
)

// instruments groups the server's HTTP instruments, built once per server
// instance at Add under the instance's name as the instrumentation scope -
// the same way a service scopes its own instruments. The provider's
// identity is stable for the life of the process, so the rebuilt echo a
// restart performs reuses them - nothing here is ever rebuilt.
type instruments struct {
	duration     httpconv.ServerRequestDuration  // http.server.request.duration, s
	active       httpconv.ServerActiveRequests   // http.server.active_requests
	requestSize  httpconv.ServerRequestBodySize  // http.server.request.body.size, By
	responseSize httpconv.ServerResponseBodySize // http.server.response.body.size, By
}

// newInstruments mints the four instruments, scoped by the server's name so
// two listeners sharing a meter keep separate instrument sets. The meter is
// optional - a listener not given one gets noop instruments, so the Metrics
// built-in runs the same way and nothing lands, the way a service without
// a meter records into noops rather than nil-checking. An error is a
// deterministic wiring fault - an invalid instrument definition - and fails
// Add, the way an invalid instrument fails a service's Init.
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

// requestMethod maps the request method onto the convention's set; anything
// outside it is _OTHER - the convention's guard against a client minting a
// new series per made-up method.
func requestMethod(method string) httpconv.RequestMethodAttr {
	switch method {
	case http.MethodConnect:
		return httpconv.RequestMethodConnect
	case http.MethodDelete:
		return httpconv.RequestMethodDelete
	case http.MethodGet:
		return httpconv.RequestMethodGet
	case http.MethodHead:
		return httpconv.RequestMethodHead
	case http.MethodOptions:
		return httpconv.RequestMethodOptions
	case http.MethodPatch:
		return httpconv.RequestMethodPatch
	case http.MethodPost:
		return httpconv.RequestMethodPost
	case http.MethodPut:
		return httpconv.RequestMethodPut
	case http.MethodTrace:
		return httpconv.RequestMethodTrace
	case "QUERY":
		return httpconv.RequestMethodQuery
	}
	return httpconv.RequestMethodOther
}
