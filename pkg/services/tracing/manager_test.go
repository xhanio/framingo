package tracing

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
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/xhanio/framingo/pkg/types/info"
	"github.com/xhanio/framingo/pkg/utils/confutil"
)

func TestDisabledByDefaultServesNoop(t *testing.T) {
	m := newManager()
	require.NoError(t, m.Init(context.Background()))
	p := m.Provider()
	require.NotNil(t, p)
	_, span := p.Tracer("test").Start(context.Background(), "op")
	assert.False(t, span.IsRecording())
	span.End()
	require.NoError(t, m.Alive(context.Background()))
	require.NoError(t, m.Start(context.Background()))
	require.NoError(t, m.Stop(context.Background()))
}

// pair with enabledCtx: Init reloads enabled from config
func newTestManager(exp sdktrace.SpanExporter) *manager {
	m := newManager()
	m.exporter = exp
	return m
}

func enabledCtx() context.Context {
	v := viper.New()
	v.Set("tracing.enabled", true)
	return confutil.WrapContext(context.Background(), confutil.New(v))
}

func flushSpans(t *testing.T, m *manager) {
	t.Helper()
	tp, ok := m.Provider().(*sdktrace.TracerProvider)
	require.True(t, ok, "flushSpans needs a live SDK provider")
	require.NoError(t, tp.ForceFlush(context.Background()))
}

func TestInjectedExporterExportsSpans(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	m := newTestManager(exp)
	require.NoError(t, m.Init(enabledCtx()))
	_, span := m.Provider().Tracer("test").Start(context.Background(), "op")
	require.True(t, span.IsRecording())
	span.End()
	// batched: nothing arrives until a flush
	assert.Empty(t, exp.GetSpans())
	flushSpans(t, m)
	spans := exp.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, "op", spans[0].Name)
}

func resourceAttr(span tracetest.SpanStub, key string) (string, bool) {
	for _, kv := range span.Resource.Attributes() {
		if string(kv.Key) == key {
			return kv.Value.AsString(), true
		}
	}
	return "", false
}

func TestSpanResourceIsBuildIdentity(t *testing.T) {
	t.Cleanup(func() {
		info.ApplicationName = ""
		info.ApplicationVersion = ""
	})
	info.ApplicationName = "exampleapp"
	info.ApplicationVersion = "v1.2.3"

	exp := tracetest.NewInMemoryExporter()
	m := newTestManager(exp)
	require.NoError(t, m.Init(enabledCtx()))
	_, span := m.Provider().Tracer("test").Start(context.Background(), "op")
	span.End()
	flushSpans(t, m)
	spans := exp.GetSpans()
	require.Len(t, spans, 1)

	name, ok := resourceAttr(spans[0], "service.name")
	require.True(t, ok)
	assert.Equal(t, "exampleapp", name)
	version, ok := resourceAttr(spans[0], "service.version")
	require.True(t, ok)
	assert.Equal(t, "v1.2.3", version)

	info.ApplicationName = ""
	info.ApplicationVersion = ""
	exp.Reset()
	require.NoError(t, m.Init(enabledCtx()))
	_, span = m.Provider().Tracer("test").Start(context.Background(), "op")
	span.End()
	flushSpans(t, m)
	spans = exp.GetSpans()
	require.Len(t, spans, 1)
	_, ok = resourceAttr(spans[0], "service.name")
	assert.False(t, ok)
	_, ok = resourceAttr(spans[0], "service.version")
	assert.False(t, ok)
}

func TestReInitSwapsProvider(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	m := newTestManager(exp)
	require.NoError(t, m.Init(enabledCtx()))
	p1 := m.Provider()

	require.NoError(t, m.Init(enabledCtx()))
	p2 := m.Provider()
	assert.NotSame(t, p1, p2)
	require.NoError(t, m.Alive(context.Background()))

	_, late := p1.Tracer("test").Start(context.Background(), "late")
	assert.False(t, late.IsRecording())
	late.End()

	exp.Reset()
	_, span := p2.Tracer("test").Start(context.Background(), "fresh")
	span.End()
	flushSpans(t, m)
	require.Len(t, exp.GetSpans(), 1)
}

func TestExporterConfigFromViper(t *testing.T) {
	v := viper.New()
	v.Set("tracing.config", map[string]any{
		"endpoint": "collector:4317",
		"insecure": true,
		"timeout":  "7s",
		"headers":  map[string]any{"authorization": "Bearer x"},
		"retry":    map[string]any{"initial_interval": "2s"},
		"whatever": "the manager does not interpret keys",
	})

	m := newManager() // disabled: Init records config and stops
	require.NoError(t, m.Init(confutil.WrapContext(context.Background(), confutil.New(v))))
	assert.Equal(t, "collector:4317", m.exporterConfig["endpoint"])
	assert.Equal(t, "the manager does not interpret keys", m.exporterConfig["whatever"])

	assert.Nil(t, newManager().exporterConfig)
}

func TestWithEnabledAndConfigOverride(t *testing.T) {
	on := viper.New()
	on.Set("tracing.enabled", true)
	m := newManager(WithEnabled(true))
	require.Error(t, m.Init(confutil.WrapContext(context.Background(), confutil.New(on))),
		"enabled with no exporter must fail")

	off := viper.New()
	off.Set("tracing.enabled", false)
	require.NoError(t, m.Init(confutil.WrapContext(context.Background(), confutil.New(off))))
	_, span := m.Provider().Tracer("test").Start(context.Background(), "op")
	assert.False(t, span.IsRecording())
}

func TestSampleRatioZeroMeansDefault(t *testing.T) {
	assert.Equal(t, 1.0, newManager(WithSampleRatio(0)).sampleRatio)
	assert.Equal(t, 0.25, newManager(WithSampleRatio(0.25)).sampleRatio)

	m := newManager(WithSampleRatio(0.25))
	require.NoError(t, m.Init(context.Background()))
	assert.Equal(t, 1.0, m.sampleRatio)

	v := viper.New()
	v.Set("tracing.sample_ratio", 0.25)
	m = newManager()
	require.NoError(t, m.Init(confutil.WrapContext(context.Background(), confutil.New(v))))
	assert.Equal(t, 0.25, m.sampleRatio)
}

func TestExporterOptionDoesNotEnable(t *testing.T) {
	// enabling would fail here: the stdout exporter is not imported
	m := newManager(WithExporter(Stdout, nil))
	require.NoError(t, m.Init(context.Background()))
	_, span := m.Provider().Tracer("test").Start(context.Background(), "op")
	assert.False(t, span.IsRecording())
}

func TestInitUnknownExporterFails(t *testing.T) {
	v := viper.New()
	v.Set("tracing.enabled", true)
	v.Set("tracing.exporter", "bogus")
	m := newManager()
	err := m.Init(confutil.WrapContext(context.Background(), confutil.New(v)))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not registered")
}

func TestInitEnabledWithoutExporterFails(t *testing.T) {
	v := viper.New()
	v.Set("tracing.enabled", true)
	m := newManager()
	require.Error(t, m.Init(confutil.WrapContext(context.Background(), confutil.New(v))))
}

func TestViperOverridesOptions(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	m := newTestManager(exp)
	off := viper.New()
	off.Set("tracing.enabled", false)
	require.NoError(t, m.Init(confutil.WrapContext(context.Background(), confutil.New(off))))
	_, span := m.Provider().Tracer("test").Start(context.Background(), "op")
	assert.False(t, span.IsRecording())
	span.End()
	assert.Empty(t, exp.GetSpans())

	on := viper.New()
	on.Set("tracing.enabled", true)
	require.NoError(t, m.Init(confutil.WrapContext(context.Background(), confutil.New(on))))
	_, span = m.Provider().Tracer("test").Start(context.Background(), "op")
	span.End()
	flushSpans(t, m)
	assert.Len(t, exp.GetSpans(), 1)
}

func TestPropagatorRoundTrip(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	m := newTestManager(exp)
	require.NoError(t, m.Init(enabledCtx()))
	ctx, span := m.Provider().Tracer("test").Start(context.Background(), "op")
	defer span.End()

	carrier := propagation.MapCarrier{}
	m.Propagator().Inject(ctx, carrier)
	require.NotEmpty(t, carrier["traceparent"])

	extracted := m.Propagator().Extract(context.Background(), carrier)
	assert.Equal(t, span.SpanContext().TraceID(), trace.SpanContextFromContext(extracted).TraceID())
}

func TestStopFlushesAndDegradesToNoop(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	m := newTestManager(exp)
	require.NoError(t, m.Init(enabledCtx()))
	_, span := m.Provider().Tracer("test").Start(context.Background(), "op")
	require.True(t, span.IsRecording())
	span.End()

	require.NoError(t, m.Stop(context.Background()))
	require.NoError(t, m.Stop(context.Background())) // idempotent

	_, span = m.Provider().Tracer("test").Start(context.Background(), "op")
	assert.False(t, span.IsRecording())
	// a stopped manager stays alive, or dependents would cascade-restart
	require.NoError(t, m.Alive(context.Background()))
}

func TestAliveFailsWhenEnabledButNeverBuilt(t *testing.T) {
	v := viper.New()
	v.Set("tracing.enabled", true)
	m := newManager()
	require.Error(t, m.Init(confutil.WrapContext(context.Background(), confutil.New(v))))
	require.Error(t, m.Alive(context.Background()))
}

// blockingExporter parks in Shutdown until released, like a stalled collector.
type blockingExporter struct {
	entered chan struct{}
	release chan struct{}
}

func (e *blockingExporter) ExportSpans(_ context.Context, _ []sdktrace.ReadOnlySpan) error {
	return nil
}

func (e *blockingExporter) Shutdown(_ context.Context) error {
	close(e.entered)
	<-e.release
	return nil
}

func TestStopIsDeliberateWhileFlushInFlight(t *testing.T) {
	exp := &blockingExporter{entered: make(chan struct{}), release: make(chan struct{})}
	m := newTestManager(exp)
	require.NoError(t, m.Init(enabledCtx()))

	stopped := make(chan error, 1)
	go func() { stopped <- m.Stop(context.Background()) }()
	<-exp.entered // the flush is in flight, the provider already retired

	assert.NoError(t, m.Alive(context.Background()))

	close(exp.release)
	require.NoError(t, <-stopped)
}

func TestStopGivesUpOnAStalledFlushOnceItsContextEnds(t *testing.T) {
	exp := &blockingExporter{entered: make(chan struct{}), release: make(chan struct{})}
	defer close(exp.release)
	m := newTestManager(exp)
	require.NoError(t, m.Init(enabledCtx()))
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	stopped := make(chan error, 1)
	go func() { stopped <- m.Stop(ctx) }()
	select {
	case err := <-stopped:
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	case <-time.After(3 * time.Second):
		t.Fatal("Stop waited out the stalled flush past its deadline")
	}
}

func TestConcurrentProbesDuringLifecycle(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	m := newTestManager(exp)
	// a populated viper makes every Init write the fields under race
	v := viper.New()
	v.Set("tracing.enabled", true)
	v.Set("tracing.sample_ratio", 1.0)
	v.Set("tracing.exporter", "race-test")
	ctx := confutil.WrapContext(context.Background(), confutil.New(v))
	require.NoError(t, m.Init(ctx))

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
				_, span := m.Provider().Tracer("test").Start(context.Background(), "op")
				span.End()
			}
		}()
	}
	for range 10 {
		require.NoError(t, m.Init(ctx))
		require.NoError(t, m.Stop(context.Background()))
		require.NoError(t, m.Init(ctx))
	}
}

func TestReInitDoesNotLeakGoroutines(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	m := newTestManager(exp)
	require.NoError(t, m.Init(enabledCtx()))
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

func TestConfigRootFollowsName(t *testing.T) {
	v := viper.New()
	v.Set("otel.sample_ratio", 0.25)
	v.Set("tracing.sample_ratio", 0.5)
	ctx := confutil.WrapContext(context.Background(), confutil.New(v))

	named := newManager(WithName("otel"))
	require.NoError(t, named.Init(ctx))
	assert.Equal(t, 0.25, named.sampleRatio)

	unnamed := newManager()
	require.NoError(t, unnamed.Init(ctx))
	assert.Equal(t, 0.5, unnamed.sampleRatio)
}
