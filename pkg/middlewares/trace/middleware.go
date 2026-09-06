// Package trace ships the framework's tracing middleware: the server span
// for every request of the servers it is enabled on. It rides the
// server-level slot - server.WithMiddlewares(trace.New(t)) at Add time,
// listed FIRST so its span covers the middlewares behind it - and activates
// per server through the middleware configs:
//
//	trace: true     # or `trace:` - the middleware takes no config
//
// With no trace entry at all the attachment stays dormant, which is how a
// probe listener serves untraced next to a traced public server. The
// middleware declares its tracing service as a dependency; the api server
// deliberately does not surface it - a collector outage degrades tracing,
// never the server - so register the middleware with the supervisor directly
// if its dependency health should show up in the stats under its own name.
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

// tracerName is the instrumentation scope of the spans this middleware
// starts.
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

// Dependencies declares the tracing service so an app that registers this
// middleware with the supervisor gets its dependency health reported under
// the middleware's own name. The api server does not consume it.
func (m *middleware) Dependencies() []common.Service {
	if m.tracer == nil {
		return nil
	}
	return []common.Service{m.tracer}
}

// Func returns the tracing middleware for one attachment. Per request it
// extracts the W3C remote parent from the headers, starts a server-kind span,
// pushes it into the request context - which the app's api.Context bridges to
// every handler and service call - and once the chain has resolved the route,
// renames the span to the low-cardinality route template with the framingo
// tid attached as api.trace_id for log<->trace correlation. Provider()
// is resolved per request on purpose: a tracing-service restart swaps
// providers without a server rebuild. On the error path the response status
// comes from the wrapped ErrorBody, because echo's HTTPErrorHandler only
// commits the response after the middleware chain has returned. On panic the
// deferred End runs during unwind before the server's recover normalizes the
// response, so that span exports with its fallback name and no error status.
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
					// the convention's set or _OTHER, never the raw method - the same
					// mapping the server's metrics use, so span and series agree
					semconv.HTTPRequestMethodKey.String(string(otelutil.RequestMethod(r.Method))),
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
				// no route matched: the info built-in short-circuited before
				// minting anything
				status = fapi.StatusCode(err)
			}
			span.SetAttributes(semconv.HTTPResponseStatusCode(status))
			// only a server failure marks the span and records the exception -
			// a 4xx is the caller's mistake, and recording every scanner's 404
			// would bloat the spans with noise
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
