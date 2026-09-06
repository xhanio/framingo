package server

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhanio/errors"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	tracemw "github.com/xhanio/framingo/pkg/middlewares/trace"
	"github.com/xhanio/framingo/pkg/utils/testutil"
)

// metricsRouter serves one route per status the built-in has to resolve:
// a plain 200, a wrapped 400 the Error built-in minted, and a bare 500.
func metricsRouter() *mockRouter {
	return &mockRouter{
		name: "metrics",
		config: []byte(`server: http
prefix: /
handlers:
  - method: GET
    path: /ok
    func: OK
  - method: GET
    path: /bad
    func: Bad
  - method: GET
    path: /fail
    func: Fail`),
		handlers: map[string]any{
			"OK":   okHandler,
			"Bad":  func(c echo.Context) error { return errors.BadRequest.Newf("bad") },
			"Fail": func(c echo.Context) error { return errors.Newf("boom") },
		},
	}
}

// waitReady polls the listener until it answers - the same probe startServer
// uses, split out so a restarted server can be waited on too. The probe hits
// an unmatched path, so every test's export also carries 404 points for "/".
func waitReady(t *testing.T, baseURL string) {
	t.Helper()
	require.Eventually(t, func() bool {
		resp, err := http.Get(baseURL + "/")
		if err != nil {
			return false
		}
		resp.Body.Close()
		return true
	}, 2*time.Second, 10*time.Millisecond)
}

// startMeasuredServer is startServer with a meter on the listener; the
// returned reader is what the test collects from.
func startMeasuredServer(t *testing.T) (baseURL string, m *manager, reader *sdkmetric.ManualReader) {
	t.Helper()
	reader, provider := testutil.NewMeterRecorder()
	port := freePort(t)
	m = testManager()
	require.NoError(t, m.Add("http",
		WithEndpoint("127.0.0.1", port, "/"),
		WithMeter(testutil.NewMeter(provider)),
	))
	require.NoError(t, m.RegisterRouters(metricsRouter()))
	require.NoError(t, m.Start(context.Background()))
	baseURL = fmt.Sprintf("http://127.0.0.1:%d", port)
	waitReady(t, baseURL)
	t.Cleanup(func() {
		dropIdleConns()
		require.NoError(t, m.Stop(true))
	})
	return
}

// dropIdleConns closes the default client's pooled connections before a
// Stop. Shutdown treats a connection that was accepted but never sent a
// request as busy for five seconds, and the client can leave exactly such
// a never-used connection in its pool when a redial races a returned one -
// a five-second stall in a test that otherwise takes milliseconds.
func dropIdleConns() {
	http.DefaultClient.CloseIdleConnections()
}

// histogramPoints collects one export and returns the named histogram's
// data points, one per attribute set; N is the instrument's value type.
func histogramPoints[N int64 | float64](t *testing.T, reader *sdkmetric.ManualReader, name string) []metricdata.HistogramDataPoint[N] {
	t.Helper()
	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	pts, found := testutil.Histogram[N](rm, name)
	require.True(t, found, "%s missing from the collected metrics", name)
	return pts
}

// pointsWith returns the data points carrying every wanted attribute.
func pointsWith[N int64 | float64](pts []metricdata.HistogramDataPoint[N], want ...attribute.KeyValue) []metricdata.HistogramDataPoint[N] {
	var out []metricdata.HistogramDataPoint[N]
	for _, p := range pts {
		ok := true
		for _, kv := range want {
			v, has := p.Attributes.Value(kv.Key)
			if !has || v.Emit() != kv.Value.Emit() {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, p)
		}
	}
	return out
}

func status(code int) attribute.KeyValue   { return attribute.Int("http.response.status_code", code) }
func route(path string) attribute.KeyValue { return attribute.String("http.route", path) }

// A matched request records one duration point carrying the convention's
// attributes plus the server's own name, and the response size beside it.
func TestMetrics_RecordsMatchedRequest(t *testing.T) {
	base, _, reader := startMeasuredServer(t)
	code, _ := httpDo(t, "GET", base+"/ok")
	require.Equal(t, http.StatusOK, code)

	pts := pointsWith(histogramPoints[float64](t, reader, "http.server.request.duration"),
		attribute.String("http.request.method", "GET"),
		attribute.String("url.scheme", "http"),
		status(200),
		route("/ok"),
		attribute.String("api.server", "http"),
	)
	require.Len(t, pts, 1)
	assert.Equal(t, uint64(1), pts[0].Count)
	assert.Greater(t, pts[0].Sum, 0.0)

	sizes := pointsWith(histogramPoints[int64](t, reader, "http.server.response.body.size"), status(200), route("/ok"))
	require.Len(t, sizes, 1)
	assert.Equal(t, int64(len("ok")), sizes[0].Sum)
}

// Status comes from the ResponseInfo the Info built-in wrote - the wrapped
// ErrorBody's, not the response's default - and a 5xx carries error.type.
func TestMetrics_StatusFromErrorBody(t *testing.T) {
	base, _, reader := startMeasuredServer(t)
	code, _ := httpDo(t, "GET", base+"/bad")
	require.Equal(t, http.StatusBadRequest, code)
	code, _ = httpDo(t, "GET", base+"/fail")
	require.Equal(t, http.StatusInternalServerError, code)

	pts := histogramPoints[float64](t, reader, "http.server.request.duration")
	bad := pointsWith(pts, status(400), route("/bad"))
	require.Len(t, bad, 1)
	_, hasErrType := bad[0].Attributes.Value("error.type")
	assert.False(t, hasErrType, "a 4xx is the caller's mistake, not an error.type")

	fail := pointsWith(pts, status(500), route("/fail"), attribute.String("error.type", "500"))
	require.Len(t, fail, 1)
}

// No route matched: Info short-circuited before minting a record, so the
// status is the returned error's and there is no http.route to carry.
func TestMetrics_UnmatchedRouteHasNoRoute(t *testing.T) {
	base, _, reader := startMeasuredServer(t)
	code, _ := httpDo(t, "GET", base+"/nope")
	require.Equal(t, http.StatusNotFound, code)

	notFound := pointsWith(histogramPoints[float64](t, reader, "http.server.request.duration"), status(404))
	require.NotEmpty(t, notFound)
	for _, p := range notFound {
		_, hasRoute := p.Attributes.Value("http.route")
		assert.False(t, hasRoute, "an unmatched request must not carry a route")
	}
}

// Every request that entered has left: the up-down counter sums to zero.
func TestMetrics_ActiveRequestsReturnToZero(t *testing.T) {
	base, _, reader := startMeasuredServer(t)
	httpDo(t, "GET", base+"/ok")
	httpDo(t, "GET", base+"/nope")

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	active, found := testutil.SumInt64(rm, "http.server.active_requests")
	require.True(t, found)
	assert.Equal(t, int64(0), active)
}

// A listener without a meter still gets instruments - noops - so the
// Metrics built-in runs unchanged and nothing lands: the health listener's
// case.
func TestMetrics_WithoutMeterUsesNoopInstruments(t *testing.T) {
	m := testManager()
	require.NoError(t, m.Add("http", WithEndpoint("127.0.0.1", 8080, "/")))
	assert.NotNil(t, m.servers["http"].inst)
}

// A restart rebuilds the echo instance but not the instruments: the same
// series keeps counting, with no duplicate point for the attribute set.
func TestMetrics_SurvivesRestart(t *testing.T) {
	base, m, reader := startMeasuredServer(t)
	httpDo(t, "GET", base+"/ok")

	dropIdleConns()
	require.NoError(t, m.Stop(true))
	require.NoError(t, m.Init(context.Background()))
	require.NoError(t, m.Start(context.Background()))
	waitReady(t, base)
	httpDo(t, "GET", base+"/ok")

	pts := pointsWith(histogramPoints[float64](t, reader, "http.server.request.duration"), status(200), route("/ok"))
	require.Len(t, pts, 1)
	assert.Equal(t, uint64(2), pts[0].Count)
}

// Records go through the request as the chain left it, so with the trace
// middleware on the same listener each point carries the span as an
// exemplar: the SDK's trace-based exemplar filter needs a sampled span in
// the recording context, which the middleware put there and never removes.
func TestMetrics_RecordsCarryTheSpan(t *testing.T) {
	reader, provider := testutil.NewMeterRecorder()
	_, tp := testutil.NewRecorder()
	port := freePort(t)
	m := testManager()
	require.NoError(t, m.Add("http",
		WithEndpoint("127.0.0.1", port, "/"),
		WithMeter(testutil.NewMeter(provider)),
		WithMiddlewares(tracemw.New(testutil.NewTracer(tp))),
		WithMiddlewareConfigs([]byte("trace: true\n")),
	))
	require.NoError(t, m.RegisterRouters(metricsRouter()))
	require.NoError(t, m.Start(context.Background()))
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitReady(t, base)
	t.Cleanup(func() {
		dropIdleConns()
		require.NoError(t, m.Stop(true))
	})

	httpDo(t, "GET", base+"/ok")
	pts := pointsWith(histogramPoints[float64](t, reader, "http.server.request.duration"), status(200), route("/ok"))
	require.Len(t, pts, 1)
	require.NotEmpty(t, pts[0].Exemplars, "the record carried no span context")
	assert.NotEmpty(t, pts[0].Exemplars[0].TraceID)
}
