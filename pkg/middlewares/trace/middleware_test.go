package trace_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhanio/errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	tracemw "github.com/xhanio/framingo/pkg/middlewares/trace"
	"github.com/xhanio/framingo/pkg/services/api/server"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"
)

// stubTracer implements model.Tracer over an in-memory exporter, keeping the
// middleware tests independent of the tracing service package.
type stubTracer struct {
	tp trace.TracerProvider
}

func (s *stubTracer) Name() string                   { return "tracing/stub" }
func (s *stubTracer) Dependencies() []common.Service { return nil }
func (s *stubTracer) Provider() trace.TracerProvider { return s.tp }
func (s *stubTracer) Propagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
}

func newStubTracer() (*stubTracer, *tracetest.InMemoryExporter) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	return &stubTracer{tp: tp}, exp
}

// mockRouter implements fapi.Router for testing.
type mockRouter struct {
	name     string
	config   []byte
	handlers map[string]any
}

func (r *mockRouter) Name() string                   { return r.name }
func (r *mockRouter) Dependencies() []common.Service { return nil }
func (r *mockRouter) Config() []byte                 { return r.config }
func (r *mockRouter) Handlers() map[string]any       { return r.handlers }

func okHandler(c echo.Context) error {
	return c.String(http.StatusOK, "ok")
}

func freePort(t *testing.T) uint {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return uint(port)
}

// startServer runs a server manager with the trace middleware attached
// server-level and activated through the middleware configs - the canonical
// wiring. A nil configs slice leaves the attachment dormant.
func startServer(t *testing.T, st *stubTracer, configs []byte, routers ...*mockRouter) (baseURL string, m server.Manager, cleanup func()) {
	t.Helper()
	port := freePort(t)
	m = server.New(server.WithLogger(log.New(log.WithLevel(1))))
	opts := []server.ServerOption{
		server.WithEndpoint("127.0.0.1", port, "/"),
		server.WithMiddlewares(tracemw.New(st)),
	}
	if configs != nil {
		opts = append(opts, server.WithMiddlewareConfigs(configs))
	}
	require.NoError(t, m.Add("http", opts...))
	for _, r := range routers {
		require.NoError(t, m.RegisterRouters(r))
	}
	require.NoError(t, m.Start(context.Background()))
	baseURL = fmt.Sprintf("http://127.0.0.1:%d", port)
	require.Eventually(t, func() bool {
		resp, err := http.Get(baseURL + "/")
		if err != nil {
			return false
		}
		resp.Body.Close()
		return true
	}, 2*time.Second, 10*time.Millisecond)
	cleanup = func() { require.NoError(t, m.Stop(true)) }
	return
}

func httpDo(t *testing.T, method, url string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(method, url, nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode, ""
}

func attrValue(attrs []attribute.KeyValue, key string) (attribute.Value, bool) {
	for _, kv := range attrs {
		if string(kv.Key) == key {
			return kv.Value, true
		}
	}
	return attribute.Value{}, false
}

func itemsRouter() *mockRouter {
	return &mockRouter{
		name: "test",
		config: []byte(`server: http
prefix: /api
handlers:
  - method: GET
    path: /items/:id
    func: GetItem`),
		handlers: map[string]any{"GetItem": okHandler},
	}
}

func TestNameAndDependencies(t *testing.T) {
	st, _ := newStubTracer()
	mw := tracemw.New(st)
	assert.Equal(t, "trace", mw.Name())

	// WithName renames the attachment - the config key follows the name, so
	// an app can dodge a collision or run two differently-named instances
	assert.Equal(t, "trace2", tracemw.New(st, tracemw.WithName("trace2")).Name())
	// an empty override keeps the default
	assert.Equal(t, "trace", tracemw.New(st, tracemw.WithName("")).Name())
	deps := mw.Dependencies()
	require.Len(t, deps, 1)
	assert.Same(t, st, deps[0])

	// a nil tracer must not leak a nil dependency - the supervisor panics on
	// nil elements inside a non-nil Dependencies slice
	assert.Nil(t, tracemw.New(nil).Dependencies())
}

func TestFuncContract(t *testing.T) {
	st, _ := newStubTracer()

	t.Run("disabled declines", func(t *testing.T) {
		fn, err := tracemw.New(st).Func(false, nil)
		require.NoError(t, err)
		assert.Nil(t, fn)
	})

	t.Run("enabled without config attaches", func(t *testing.T) {
		fn, err := tracemw.New(st).Func(true, nil)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("a config block fails registration", func(t *testing.T) {
		_, err := tracemw.New(st).Func(true, []byte("skip: something\n"))
		require.Error(t, err)
	})

	t.Run("a nil tracer fails registration", func(t *testing.T) {
		_, err := tracemw.New(nil).Func(true, nil)
		require.Error(t, err)
	})
}

func TestDormantWithoutConfigEntry(t *testing.T) {
	// the health-listener story: attached in code, but with no "trace" entry
	// in the server's middleware configs the attachment stays dormant
	st, exp := newStubTracer()
	base, _, cleanup := startServer(t, st, nil, itemsRouter())
	defer cleanup()
	exp.Reset()

	code, _ := httpDo(t, "GET", base+"/api/items/42")
	require.Equal(t, http.StatusOK, code)
	assert.Empty(t, exp.GetSpans())
}

func TestMatchedRouteSpan(t *testing.T) {
	st, exp := newStubTracer()
	base, _, cleanup := startServer(t, st, []byte("trace:\n"), itemsRouter())
	defer cleanup()
	exp.Reset() // drop the readiness-probe spans

	code, _ := httpDo(t, "GET", base+"/api/items/42")
	require.Equal(t, http.StatusOK, code)

	spans := exp.GetSpans()
	require.Len(t, spans, 1)
	span := spans[0]
	assert.Equal(t, "GET /api/items/:id", span.Name)
	assert.Equal(t, trace.SpanKindServer, span.SpanKind)
	if v, ok := attrValue(span.Attributes, "http.route"); assert.True(t, ok) {
		assert.Equal(t, "/api/items/:id", v.AsString())
	}
	if v, ok := attrValue(span.Attributes, "url.path"); assert.True(t, ok) {
		assert.Equal(t, "/api/items/42", v.AsString())
	}
	if v, ok := attrValue(span.Attributes, "http.request.method"); assert.True(t, ok) {
		assert.Equal(t, "GET", v.AsString())
	}
	if v, ok := attrValue(span.Attributes, "http.response.status_code"); assert.True(t, ok) {
		assert.EqualValues(t, http.StatusOK, v.AsInt64())
	}
	if v, ok := attrValue(span.Attributes, "framingo.trace_id"); assert.True(t, ok) {
		assert.NotEmpty(t, v.AsString())
	}
	assert.NotEqual(t, codes.Error, span.Status.Code)
}

func TestRemoteParentExtracted(t *testing.T) {
	st, exp := newStubTracer()
	base, _, cleanup := startServer(t, st, []byte("trace:\n"), itemsRouter())
	defer cleanup()
	exp.Reset()

	const traceID = "4bf92f3577b34da6a3ce929d0e0e4736"
	const parentID = "00f067aa0ba902b7"
	req, err := http.NewRequest(http.MethodGet, base+"/api/items/42", nil)
	require.NoError(t, err)
	req.Header.Set("traceparent", fmt.Sprintf("00-%s-%s-01", traceID, parentID))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()

	spans := exp.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, traceID, spans[0].SpanContext.TraceID().String())
	assert.Equal(t, parentID, spans[0].Parent.SpanID().String())
	assert.True(t, spans[0].Parent.IsRemote())
}

func TestHandlerSeesSpan(t *testing.T) {
	seeing := func(c echo.Context) error {
		if trace.SpanContextFromContext(c.Request().Context()).IsValid() {
			return c.String(http.StatusOK, "traced")
		}
		return c.String(http.StatusOK, "untraced")
	}
	st, _ := newStubTracer()
	base, _, cleanup := startServer(t, st, []byte("trace:\n"), &mockRouter{
		name: "test",
		config: []byte(`server: http
prefix: /api
handlers:
  - method: GET
    path: /probe
    func: Probe`),
		handlers: map[string]any{"Probe": seeing},
	})
	defer cleanup()

	req, err := http.NewRequest(http.MethodGet, base+"/api/probe", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	buf := make([]byte, 16)
	n, _ := resp.Body.Read(buf)
	assert.Equal(t, "traced", string(buf[:n]))
}

func TestNoRouteKeepsFallbackName(t *testing.T) {
	st, exp := newStubTracer()
	base, _, cleanup := startServer(t, st, []byte("trace:\n"), itemsRouter())
	defer cleanup()
	exp.Reset()

	code, _ := httpDo(t, "GET", base+"/nope")
	require.Equal(t, http.StatusNotFound, code)

	spans := exp.GetSpans()
	require.Len(t, spans, 1)
	span := spans[0]
	assert.Equal(t, "HTTP GET", span.Name)
	_, hasRoute := attrValue(span.Attributes, "http.route")
	assert.False(t, hasRoute)
	if v, ok := attrValue(span.Attributes, "http.response.status_code"); assert.True(t, ok) {
		assert.EqualValues(t, http.StatusNotFound, v.AsInt64())
	}
	assert.NotEqual(t, codes.Error, span.Status.Code)
	for _, ev := range span.Events {
		assert.NotEqual(t, "exception", ev.Name)
	}
}

func TestServerErrorMarksSpan(t *testing.T) {
	failing := func(c echo.Context) error {
		return errors.Newf("boom")
	}
	st, exp := newStubTracer()
	base, _, cleanup := startServer(t, st, []byte("trace:\n"), &mockRouter{
		name: "test",
		config: []byte(`server: http
prefix: /api
handlers:
  - method: GET
    path: /fail
    func: Fail`),
		handlers: map[string]any{"Fail": failing},
	})
	defer cleanup()
	exp.Reset()

	code, _ := httpDo(t, "GET", base+"/api/fail")
	require.Equal(t, http.StatusInternalServerError, code)

	spans := exp.GetSpans()
	require.Len(t, spans, 1)
	span := spans[0]
	assert.Equal(t, "GET /api/fail", span.Name)
	if v, ok := attrValue(span.Attributes, "http.response.status_code"); assert.True(t, ok) {
		assert.EqualValues(t, http.StatusInternalServerError, v.AsInt64())
	}
	assert.Equal(t, codes.Error, span.Status.Code)
	var recorded bool
	for _, ev := range span.Events {
		if ev.Name == "exception" {
			recorded = true
		}
	}
	assert.True(t, recorded, "the wrapped error must be recorded on the span")
}

func TestSurvivesRestartRebuild(t *testing.T) {
	st, exp := newStubTracer()
	base, m, cleanup := startServer(t, st, []byte("trace:\n"), itemsRouter())
	defer cleanup()

	require.NoError(t, m.Stop(true))
	require.NoError(t, m.Init(context.Background()))
	require.NoError(t, m.Start(context.Background()))
	require.Eventually(t, func() bool {
		resp, err := http.Get(base + "/api/items/42")
		if err != nil {
			return false
		}
		resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 2*time.Second, 10*time.Millisecond)

	exp.Reset()
	code, _ := httpDo(t, "GET", base+"/api/items/42")
	require.Equal(t, http.StatusOK, code)
	spans := exp.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, "GET /api/items/:id", spans[0].Name)
}
