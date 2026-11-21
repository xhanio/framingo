// Package trace is the server-span middleware; attach it first and enable it per server with a `trace` middleware config entry.
package trace

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/xhanio/errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	oteltrace "go.opentelemetry.io/otel/trace"

	fapi "github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/model"
	"github.com/xhanio/framingo/pkg/utils/otelutil"
)

const tracerName = "github.com/xhanio/framingo/pkg/middlewares/trace"

var _ fapi.Middleware = (*middleware)(nil)

type middleware struct {
	name   string
	tracer model.Tracer
}

func New(t model.Tracer, opts ...Option) fapi.Middleware {
	m := &middleware{
		tracer: t,
	}
	m.applyOptions(opts...)
	m.setDefaults()
	return m
}

func (m *middleware) Name() string {
	return m.name
}

// Dependencies is for registering with the supervisor directly; the api server ignores it.
func (m *middleware) Dependencies() []common.Service {
	return []common.Service{m.tracer}
}

// Provider() is resolved per request so a tracing-service restart needs no rebuild;
// the error status comes from ErrorBody since echo commits the response after the chain returns.
func (m *middleware) Func(enabled bool, config []byte) (func(echo.HandlerFunc) echo.HandlerFunc, error) {
	if !enabled {
		return nil, nil
	}
	if m.tracer == nil {
		return nil, errors.Newf("trace middleware has no tracer")
	}
	if config != nil {
		return nil, errors.Newf("trace takes no config")
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			r := c.Request()
			ctx := m.tracer.Propagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
			ctx, span := m.tracer.Provider().Tracer(tracerName).Start(ctx, "HTTP "+r.Method,
				oteltrace.WithSpanKind(oteltrace.SpanKindServer),
				oteltrace.WithAttributes(
					// same method mapping as the server metrics
					semconv.HTTPRequestMethodKey.String(string(otelutil.RequestMethod(r.Method))),
					semconv.NetworkProtocolVersion(otelutil.ProtocolVersion(r)),
					semconv.URLPath(r.URL.Path),
					semconv.ClientAddress(c.RealIP()),
				),
			)
			defer span.End()
			c.SetRequest(r.WithContext(ctx))

			err := next(c)

			status := c.Response().Status
			if req, ok := c.Get(fapi.ContextKeyRequestInfo).(*fapi.RequestInfo); ok && req != nil {
				span.SetName(req.Method + " " + req.RawPath)
				span.SetAttributes(
					semconv.HTTPRoute(req.RawPath),
					attribute.String("api.trace_id", req.TraceID),
				)
			}
			ae, _ := c.Get(fapi.ContextKeyError).(*fapi.ErrorBody)
			if ae != nil {
				status = ae.Status
			} else if err != nil {
				// no route matched, so no ErrorBody was minted
				status = fapi.StatusCode(err)
			}
			span.SetAttributes(semconv.HTTPResponseStatusCode(status))
			// only 5xx marks the span; 4xx would be noise
			if status >= http.StatusInternalServerError {
				span.SetStatus(codes.Error, http.StatusText(status))
				if ae != nil && ae.Origin != nil {
					span.RecordError(ae.Origin)
				} else if err != nil {
					span.RecordError(err)
				}
			}
			return err
		}
	}, nil
}
