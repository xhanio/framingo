package server

import (
	"net/http"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/xhanio/errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/semconv/v1.43.0/httpconv"

	"github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/otelutil"
)

// middlewares holds the server's lifecycle middlewares - metrics, recover, logger,
// info, error. They take no config and answer to no name, so they stay plain
// functions rather than paying for the api.Middleware contract; everything
// with a name and a config is the app's, attached through router.yaml or
// WithMiddlewares.
type middlewares struct {
	server *server
}

// newMiddleware creates a new middlewares instance for the given server
func newMiddleware(srv *server) *middlewares {
	return &middlewares{
		server: srv,
	}
}

// Error middlewares wraps and handles errors from handlers
func (mw *middlewares) Error(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		err := next(c)
		if err != nil {
			apiError := api.WrapError(err, c)
			apiError.Source = mw.server.Name()
			c.Set(api.ContextKeyError, apiError)
			return apiError
		}
		return nil
	}
}

// Info middlewares extracts request information and injects it into context
func (mw *middlewares) Info(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		req, found := mw.server.requestInfo(c)
		if !found {
			return errors.NotFound.Newf("failed to look up handler %s", c.Request().RequestURI)
		}
		c.Set(api.ContextKeyRequestInfo, req)
		c.Set(api.ContextKeyTrace, req.TraceID)
		err := next(c)
		resp := mw.server.responseInfo(req.StartedAt, c)
		c.Set(api.ContextKeyResponseInfo, resp)
		return err
	}
}

// Logger middlewares logs request and response information
func (mw *middlewares) Logger(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		err := next(c)
		req, ok := c.Get(common.ContextKeyAPIRequestInfo).(*api.RequestInfo)
		if !ok || req == nil {
			return errors.NotFound.Newf("failed to look up handler %s", c.Request().URL.EscapedPath())
		}
		resp, ok := c.Get(common.ContextKeyAPIResponseInfo).(*api.ResponseInfo)
		if !ok || resp == nil {
			return errors.Newf("failed to get response from %s", c.Request().RequestURI)
		}
		if req.Poll {
			// TODO: stack polling api logs
		} else {
			mw.server.print(req, resp)
		}
		return err
	}
}

// Recover middlewares recovers from panics and converts them to errors
func (mw *middlewares) Recover(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		defer func() {
			if r := recover(); r != nil {
				mw.server.log.Error(string(debug.Stack()))
				var err error
				switch e := r.(type) {
				case errors.Error:
					err = errors.Wrapf(e, "!! recover from panic")
				case errors.Category:
					err = errors.Wrapf(e, "!! recover from panic")
				default:
					err = errors.Newf("!! recover from panic: %v", r)
				}
				c.Error(err)
			}
		}()
		return next(c)
	}
}

// Metrics records the request through the server's instruments: duration,
// active requests and both body sizes, attributed with method, scheme,
// status, the route template and this server's name. Status and size come
// from the ResponseInfo the Info built-in wrote - already the wrapped
// ErrorBody's when one was set, and what the request log prints, so metrics
// and logs agree. An unmatched route or a request an outer middleware
// refused left none; there the status is the returned error's, which the
// error handler commits only after the chain returns, or the committed
// response's. Duration is measured here, outermost, so it brackets the
// whole chain. The response size is recorded only for a response the chain
// itself wrote - an error body the error handler writes afterwards has no
// size yet.
func (mw *middlewares) Metrics(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		start := time.Now()
		r := c.Request()
		method := otelutil.RequestMethod(r.Method)
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		inst := mw.server.inst
		inst.active.Add(r.Context(), 1, method, scheme)
		defer inst.active.Add(r.Context(), -1, method, scheme)

		err := next(c)

		// the request as the chain left it: the trace middleware, when it
		// ran, swapped in one whose context carries the span - and never
		// restores the original - so recording through it links exemplars
		// to the trace. The span has ended by now; its context has not.
		ctx := c.Request().Context()
		status, size := c.Response().Status, c.Response().Size
		if resp, ok := c.Get(api.ContextKeyResponseInfo).(*api.ResponseInfo); ok && resp != nil {
			status, size = resp.Status, int64(resp.Size)
		} else if err != nil {
			status = api.StatusCode(err)
		}
		attrs := []attribute.KeyValue{
			inst.duration.AttrResponseStatusCode(status),
			inst.duration.AttrNetworkProtocolVersion(otelutil.ProtocolVersion(r)),
			attribute.String("api.server", mw.server.name),
		}
		if req, ok := c.Get(api.ContextKeyRequestInfo).(*api.RequestInfo); ok && req != nil {
			// the template, never the raw path - one series per route
			attrs = append(attrs, inst.duration.AttrRoute(req.RawPath))
		}
		// only a server failure is an error in the convention's eyes; a 4xx
		// is the caller's mistake, and without an exception the status code
		// string is what error.type carries
		if status >= http.StatusInternalServerError {
			attrs = append(attrs, inst.duration.AttrErrorType(httpconv.ErrorTypeAttr(strconv.Itoa(status))))
		}
		inst.duration.Record(ctx, time.Since(start).Seconds(), method, scheme, attrs...)
		if r.ContentLength >= 0 {
			inst.requestSize.Record(ctx, r.ContentLength, method, scheme, attrs...)
		}
		// only a response the chain itself wrote has a size: an error body the
		// error handler writes after the chain returns has none yet, and a zero
		// would only skew the histogram
		if c.Response().Committed {
			inst.responseSize.Record(ctx, size, method, scheme, attrs...)
		}
		return err
	}
}
