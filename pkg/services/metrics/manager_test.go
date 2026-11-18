package metrics

import (
	"context"
	"io"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/xhanio/framingo/pkg/types/info"
	"github.com/xhanio/framingo/pkg/utils/confutil"
	"github.com/xhanio/framingo/pkg/utils/testutil"
)

// memExporter stands in for tracetest.InMemoryExporter, which the metric SDK lacks.
type memExporter struct {
	mu       sync.Mutex
	exports  []metricdata.ResourceMetrics
	shutdown bool
}

func (e *memExporter) Temporality(k sdkmetric.InstrumentKind) metricdata.Temporality {
	return sdkmetric.DefaultTemporalitySelector(k)
}

func (e *memExporter) Aggregation(k sdkmetric.InstrumentKind) sdkmetric.Aggregation {
	return sdkmetric.DefaultAggregationSelector(k)
}

func (e *memExporter) Export(_ context.Context, rm *metricdata.ResourceMetrics) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.exports = append(e.exports, *rm)
	return nil
}

func (e *memExporter) ForceFlush(context.Context) error { return nil }

func (e *memExporter) Shutdown(context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.shutdown = true
	return nil
}

func (e *memExporter) count() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.exports)
}

func (e *memExporter) last() metricdata.ResourceMetrics {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.exports[len(e.exports)-1]
}

func (e *memExporter) isShutdown() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.shutdown
}

// pair with enabledCtx: Init reloads enabled from config
func newTestManager(exp sdkmetric.Exporter) *manager {
	m := newManager()
	m.exporter = exp
	return m
}

func enabledCtx(kv ...any) context.Context {
	v := viper.New()
	v.Set("metrics.enabled", true)
	for i := 0; i+1 < len(kv); i += 2 {
		v.Set(kv[i].(string), kv[i+1])
	}
	return confutil.WrapContext(context.Background(), confutil.New(v))
}

func disabledCtx(kv ...any) context.Context {
	v := viper.New()
	for i := 0; i+1 < len(kv); i += 2 {
		v.Set(kv[i].(string), kv[i+1])
	}
	return confutil.WrapContext(context.Background(), confutil.New(v))
}

func sumValue(t *testing.T, rm metricdata.ResourceMetrics, name string) int64 {
	t.Helper()
	total, found := testutil.SumInt64(rm, name)
	require.True(t, found, "expected an int64 Sum for %s in the export", name)
	return total
}

func TestProviderIsStableAndAlwaysReal(t *testing.T) {
	m := newManager()
	p1 := m.Provider()
	require.NotNil(t, p1)
	require.NoError(t, m.Init(context.Background()))
	assert.Same(t, p1, m.Provider())
	require.NoError(t, m.Alive(context.Background()))
	require.NoError(t, m.Start(context.Background()))
	require.NoError(t, m.Stop(context.Background()))
	assert.Same(t, p1, m.Provider())
}

func TestDisabledAggregatesButNeverExports(t *testing.T) {
	exp := &memExporter{}
	m := newTestManager(exp)
	require.NoError(t, m.Init(context.Background())) // no config: disabled
	c, err := m.Provider().Meter("test").Int64Counter("test.count")
	require.NoError(t, err)
	c.Add(context.Background(), 1)
	require.NoError(t, m.export(context.Background()))
	assert.Zero(t, exp.count(), "disabled must install no exporter")
}

func TestInjectedExporterReceivesMetrics(t *testing.T) {
	exp := &memExporter{}
	m := newTestManager(exp)
	require.NoError(t, m.Init(enabledCtx()))
	c, err := m.Provider().Meter("test").Int64Counter("test.count")
	require.NoError(t, err)
	c.Add(context.Background(), 3)
	require.NoError(t, m.export(context.Background()))
	require.Equal(t, 1, exp.count())
	assert.Equal(t, int64(3), sumValue(t, exp.last(), "test.count"))
}

func TestInstrumentSurvivesRestart(t *testing.T) {
	expA := &memExporter{}
	m := newTestManager(expA)
	require.NoError(t, m.Init(enabledCtx()))
	c, err := m.Provider().Meter("test").Int64Counter("test.count")
	require.NoError(t, err)
	c.Add(context.Background(), 1)

	expB := &memExporter{}
	m.exporter = expB
	require.NoError(t, m.Init(enabledCtx()))
	assert.True(t, expA.isShutdown(), "the retired exporter must be shut down")

	c.Add(context.Background(), 1)
	require.NoError(t, m.export(context.Background()))
	require.Equal(t, 1, expB.count())
	assert.Equal(t, int64(2), sumValue(t, expB.last(), "test.count"))
	assert.Zero(t, expA.count(), "no export may reach the retired exporter")
}

func TestInitEnabledWithoutExporterFails(t *testing.T) {
	m := newManager()
	require.Error(t, m.Init(enabledCtx()))
	require.Error(t, m.Alive(context.Background()),
		"enabled with no exporter built is the one wiring fault Alive reports")
}

func TestInitUnknownExporterFails(t *testing.T) {
	m := newManager()
	err := m.Init(enabledCtx("metrics.exporter", "bogus"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not registered")
}

func TestExporterConfigFromViper(t *testing.T) {
	m := newManager() // disabled: Init records config and stops
	require.NoError(t, m.Init(disabledCtx("metrics.config", map[string]any{
		"endpoint": "collector:4317",
		"whatever": "the manager does not interpret keys",
	})))
	assert.Equal(t, "collector:4317", m.exporterConfig["endpoint"])
	assert.Equal(t, "the manager does not interpret keys", m.exporterConfig["whatever"])
	assert.Nil(t, newManager().exporterConfig)
}

func TestExporterOptionDoesNotEnable(t *testing.T) {
	// enabling would fail here: the stdout exporter is not imported
	m := newManager(WithExporter(Stdout, nil))
	require.NoError(t, m.Init(context.Background()))
}

func TestIntervalZeroMeansDefault(t *testing.T) {
	assert.Equal(t, defaultInterval, newManager(WithInterval(0)).interval)
	assert.Equal(t, 5*time.Second, newManager(WithInterval(5*time.Second)).interval)

	m := newManager(WithInterval(5 * time.Second))
	require.NoError(t, m.Init(context.Background()))
	assert.Equal(t, defaultInterval, m.interval)

	m = newManager()
	require.NoError(t, m.Init(disabledCtx("metrics.interval", "15s")))
	assert.Equal(t, 15*time.Second, m.interval)
}

func TestViperOverridesOptions(t *testing.T) {
	exp := &memExporter{}
	m := newTestManager(exp)
	off := viper.New()
	off.Set("metrics.enabled", false)
	require.NoError(t, m.Init(confutil.WrapContext(context.Background(), confutil.New(off))))
	live, _ := m.controller.state()
	assert.False(t, live)

	require.NoError(t, m.Init(enabledCtx()))
	c, err := m.Provider().Meter("test").Int64Counter("test.toggle")
	require.NoError(t, err)
	c.Add(context.Background(), 1)
	require.NoError(t, m.export(context.Background()))
	assert.Equal(t, 1, exp.count())
}

func TestResourceIsBuildIdentity(t *testing.T) {
	t.Cleanup(func() {
		info.ApplicationName = ""
		info.ApplicationVersion = ""
	})
	info.ApplicationName = "exampleapp"
	info.ApplicationVersion = "v1.2.3"

	// the resource is fixed at construction, so set info before newManager
	exp := &memExporter{}
	m := newTestManager(exp)
	require.NoError(t, m.Init(enabledCtx()))
	c, err := m.Provider().Meter("test").Int64Counter("test.count")
	require.NoError(t, err)
	c.Add(context.Background(), 1)
	require.NoError(t, m.export(context.Background()))
	require.Equal(t, 1, exp.count())

	attrs := map[string]string{}
	for _, kv := range exp.last().Resource.Attributes() {
		attrs[string(kv.Key)] = kv.Value.AsString()
	}
	assert.Equal(t, "exampleapp", attrs["service.name"])
	assert.Equal(t, "v1.2.3", attrs["service.version"])
}

func TestLoopExportsAtInterval(t *testing.T) {
	exp := &memExporter{}
	m := newTestManager(exp)
	require.NoError(t, m.Init(enabledCtx("metrics.interval", "20ms")))
	c, err := m.Provider().Meter("test").Int64Counter("test.count")
	require.NoError(t, err)
	c.Add(context.Background(), 1)
	require.NoError(t, m.Start(context.Background()))
	require.Eventually(t, func() bool { return exp.count() >= 1 },
		2*time.Second, 10*time.Millisecond, "the loop must export on its own")
	require.NoError(t, m.Stop(context.Background()))
}

func TestStopFlushesAndShutsDown(t *testing.T) {
	exp := &memExporter{}
	m := newTestManager(exp)
	require.NoError(t, m.Init(enabledCtx())) // default interval: no tick fires
	c, err := m.Provider().Meter("test").Int64Counter("test.count")
	require.NoError(t, err)
	require.NoError(t, m.Start(context.Background()))
	c.Add(context.Background(), 7)

	require.NoError(t, m.Stop(context.Background()))
	require.NoError(t, m.Stop(context.Background())) // idempotent

	require.Equal(t, 1, exp.count())
	assert.Equal(t, int64(7), sumValue(t, exp.last(), "test.count"))
	assert.True(t, exp.isShutdown())

	// a stopped manager stays alive, or dependents would cascade-restart
	require.NoError(t, m.Alive(context.Background()))
}

func TestReInitDoesNotLeakGoroutines(t *testing.T) {
	exp := &memExporter{}
	m := newTestManager(exp)
	require.NoError(t, m.Init(enabledCtx()))
	require.NoError(t, m.Start(context.Background()))
	runtime.GC()
	before := runtime.NumGoroutine()
	for range 10 {
		require.NoError(t, m.Init(enabledCtx()))
	}
	require.NoError(t, m.Stop(context.Background()))
	time.Sleep(50 * time.Millisecond)
	runtime.GC()
	after := runtime.NumGoroutine()
	assert.LessOrEqual(t, after-before, 2, "Init cycles must not leave goroutines behind")
}

func TestConcurrentProbesDuringLifecycle(t *testing.T) {
	exp := &memExporter{}
	m := newTestManager(exp)
	// a populated viper makes every Init write the fields under race
	ctx := enabledCtx("metrics.interval", "5ms", "metrics.exporter", "race-test")
	require.NoError(t, m.Init(ctx))
	c, err := m.Provider().Meter("test").Int64Counter("test.count")
	require.NoError(t, err)

	done := make(chan struct{})
	var wg sync.WaitGroup
	defer func() { close(done); wg.Wait() }()
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
				}
				//nolint:errcheck // transient states are fine; the race detector is the assertion
				m.Alive(context.Background())
				m.Info(io.Discard, true)
				c.Add(context.Background(), 1)
			}
		}()
	}
	for range 10 {
		require.NoError(t, m.Init(ctx))
		require.NoError(t, m.Start(context.Background()))
		require.NoError(t, m.Stop(context.Background()))
		require.NoError(t, m.Init(ctx))
	}
}

func TestConfigRootFollowsName(t *testing.T) {
	v := viper.New()
	v.Set("telemetry.interval", "5s")
	v.Set("metrics.interval", "9s")
	ctx := confutil.WrapContext(context.Background(), confutil.New(v))

	named := newManager(WithName("telemetry"))
	require.NoError(t, named.Init(ctx))
	assert.Equal(t, 5*time.Second, named.currentInterval())

	unnamed := newManager()
	require.NoError(t, unnamed.Init(ctx))
	assert.Equal(t, 9*time.Second, unnamed.currentInterval())
}
