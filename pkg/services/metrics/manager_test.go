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

// memExporter captures exported metrics in memory - the metrics analogue of
// tracetest.InMemoryExporter, which the metric SDK does not ship.
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

// newTestManager injects exp directly on the concrete manager - the package
// tests' seam, since instance injection has no public option. Pair it with
// enabledCtx: Init reloads from config, so setting enabled here would not
// survive the call.
func newTestManager(exp sdkmetric.Exporter) *manager {
	m := newManager()
	m.exporter = exp
	return m
}

// enabledCtx turns metrics on the way production does - through config, the
// only authority Init consults. Extra key-value pairs land on the same viper.
func enabledCtx(kv ...any) context.Context {
	v := viper.New()
	v.Set("metrics.enabled", true)
	for i := 0; i+1 < len(kv); i += 2 {
		v.Set(kv[i].(string), kv[i+1])
	}
	return confutil.WrapContext(context.Background(), confutil.New(v))
}

// disabledCtx builds a config context without turning metrics on.
func disabledCtx(kv ...any) context.Context {
	v := viper.New()
	for i := 0; i+1 < len(kv); i += 2 {
		v.Set(kv[i].(string), kv[i+1])
	}
	return confutil.WrapContext(context.Background(), confutil.New(v))
}

// sumValue digs the int64 sum of the named instrument out of one export.
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
	// disabled Init must not replace the provider - that is the whole point
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

// TestInstrumentSurvivesRestart is the design's reason to exist: a counter
// created before a restart must record into the exporter installed after
// it, because the provider identity never changes - only the export path.
func TestInstrumentSurvivesRestart(t *testing.T) {
	expA := &memExporter{}
	m := newTestManager(expA)
	require.NoError(t, m.Init(enabledCtx()))
	c, err := m.Provider().Meter("test").Int64Counter("test.count")
	require.NoError(t, err)
	c.Add(context.Background(), 1)

	// the restart: a fresh Init retires expA and installs expB
	expB := &memExporter{}
	m.exporter = expB
	require.NoError(t, m.Init(enabledCtx()))
	assert.True(t, expA.isShutdown(), "the retired exporter must be shut down")

	// the SAME cached instrument keeps recording - cumulative state included
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
	// metrics.config is an opaque kv block: the manager records it verbatim
	// and hands it to the exporter factory, which strict-decodes its own
	// struct - the same contract as tracing.config.
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
	// configuring where metrics would go is not the same act as turning the
	// service on: metrics.enabled is the only thing that does it. Were it
	// otherwise this Init would reach the registry and fail on the
	// unimported stdout exporter.
	m := newManager(WithExporter(Stdout, nil))
	require.NoError(t, m.Init(context.Background()))
}

func TestIntervalZeroMeansDefault(t *testing.T) {
	// Go cannot tell an unset interval from an explicit zero, so the option
	// reads zero as "use the default" - letting a wiring layer pass an
	// absent config key straight through without an IsSet dance
	assert.Equal(t, defaultInterval, newManager(WithInterval(0)).interval)
	assert.Equal(t, 5*time.Second, newManager(WithInterval(5*time.Second)).interval)

	// Init reloads through the same option, so the rule holds there too
	m := newManager(WithInterval(5 * time.Second))
	require.NoError(t, m.Init(context.Background()))
	assert.Equal(t, defaultInterval, m.interval)

	m = newManager()
	require.NoError(t, m.Init(disabledCtx("metrics.interval", "15s")))
	assert.Equal(t, 15*time.Second, m.interval)
}

func TestViperOverridesOptions(t *testing.T) {
	// a set key wins over the constructed state: enabled=false switches an
	// injected-exporter manager off...
	exp := &memExporter{}
	m := newTestManager(exp)
	off := viper.New()
	off.Set("metrics.enabled", false)
	require.NoError(t, m.Init(confutil.WrapContext(context.Background(), confutil.New(off))))
	live, _ := m.controller.state()
	assert.False(t, live)

	// ...and a later restart with enabled=true switches it back on
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

	// the resource is fixed at construction, so info must be set before
	// newManager - unlike tracing, where Init rebuilds the pipeline
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

	// the final flush carried the last window out before shutdown
	require.Equal(t, 1, exp.count())
	assert.Equal(t, int64(7), sumValue(t, exp.last(), "test.count"))
	assert.True(t, exp.isShutdown())

	// after a deliberate stop liveness must NOT fail: the monitor probes
	// stopped services through dependents' chains, and a failing Alive
	// would cascade restarts
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
	// a populated viper makes every Init actually rewrite the config fields,
	// which is the write side of the race this test guards against
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

// A named manager reads its keys under that name and ignores the package
// root; an unnamed one reads metrics.*. Both stay disabled, so Init records
// the config and stops - interval is the observable.
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
