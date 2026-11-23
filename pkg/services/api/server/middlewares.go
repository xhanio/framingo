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

type middlewares struct {
	server *server
}

func newMiddleware(srv *server) *middlewares {
	return &middlewares{
		server: srv,
	}
}

func (mw *middlewares) Error(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		err := next(c)
		if err != nil {
			apiError := api.WrapError(err)
			apiError.Source = mw.server.Name()
			c.Set(api.ContextKeyError, apiError)
			return apiError
		}
		return nil
	}
}

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

func (mw *middlewares) Recover(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		defer func() {
			if r := recover(); r != nil {
				mw.server.log.Error(string(debug.Stack()))
				var err error
				switch e := r.(type) {
				case error: // keeps its category, whichever package wrapped it
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

// status and size come from Info's ResponseInfo so metrics and logs agree; else from the returned error
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

		// the trace middleware's swapped-in request context links exemplars to the span
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
			// the template, never the raw path: one series per route
			attrs = append(attrs, inst.duration.AttrRoute(req.RawPath))
		}
		if status >= http.StatusInternalServerError {
			attrs = append(attrs, inst.duration.AttrErrorType(httpconv.ErrorTypeAttr(strconv.Itoa(status))))
		}
		inst.duration.Record(ctx, time.Since(start).Seconds(), method, scheme, attrs...)
		if r.ContentLength >= 0 {
			inst.requestSize.Record(ctx, r.ContentLength, method, scheme, attrs...)
		}
		// the error handler writes its body after the chain, so an uncommitted size is a bogus zero
		if c.Response().Committed {
			inst.responseSize.Record(ctx, size, method, scheme, attrs...)
		}
		return err
	}
}
