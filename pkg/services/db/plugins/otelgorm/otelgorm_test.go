package otelgorm

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/embedded"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"gorm.io/gorm"
	"gorm.io/plugin/opentelemetry/tracing"

	"github.com/xhanio/framingo/pkg/services/db"
	_ "github.com/xhanio/framingo/pkg/services/db/drivers/sqlite"
	"github.com/xhanio/framingo/pkg/types/model"
	"github.com/xhanio/framingo/pkg/utils/confutil"
	"github.com/xhanio/framingo/pkg/utils/testutil"
)

func newPlugin(t *testing.T, tr model.Tracer, opts ...tracing.Option) gorm.Plugin {
	t.Helper()
	p, err := New(tr, opts...)
	require.NoError(t, err)
	return p
}

// max_open=1 keeps every query on the one :memory: connection
func setupDB(t *testing.T, opts ...db.Option) db.Manager {
	t.Helper()

	v := viper.New()
	v.Set("db.connection.max_open", 1)
	v.Set("db.connection.max_idle", 1)
	v.Set("db.connection.exec_timeout", 2*time.Second)
	ctx := confutil.WrapContext(context.Background(), confutil.New(v))

	mgr := db.New(append([]db.Option{
		db.WithType(db.SQLite),
		db.WithDataSource(db.Source{}),
	}, opts...)...)
	require.NoError(t, mgr.Init(ctx))
	require.NoError(t, mgr.ORM().Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL)`).Error)
	return mgr
}

func spanNamed(t *testing.T, spans tracetest.SpanStubs, name string) tracetest.SpanStub {
	t.Helper()
	for _, s := range spans {
		if s.Name == name {
			return s
		}
	}
	require.FailNowf(t, "span not found", "no span named %q among %d exported spans", name, len(spans))
	return tracetest.SpanStub{}
}

func TestQueryFromContextNestsUnderCallerSpan(t *testing.T) {
	exp, tp := testutil.NewRecorder()
	tr := testutil.NewTracer(tp)
	mgr := setupDB(t, db.WithPlugins(newPlugin(t, tr)))
	exp.Reset()

	ctx, parent := tp.Tracer("test").Start(context.Background(), "request")
	require.NoError(t, mgr.FromContext(ctx).Exec(`INSERT INTO items (name) VALUES ('a')`).Error)
	parent.End()

	dbSpan := spanNamed(t, exp.GetSpans(), "gorm.Raw")
	assert.Equal(t, parent.SpanContext().TraceID(), dbSpan.SpanContext.TraceID())
	assert.Equal(t, parent.SpanContext().SpanID(), dbSpan.Parent.SpanID())
}

// upstream resolves its tracer once, so the captured handle must stay live
func TestProviderSwapTakesEffectWithoutReinstall(t *testing.T) {
	retiredExp, retiredTP := testutil.NewRecorder()
	tr := testutil.NewTracer(retiredTP)
	mgr := setupDB(t, db.WithPlugins(newPlugin(t, tr)))

	currentExp, currentTP := testutil.NewRecorder()
	tr.Swap(currentTP)
	retiredExp.Reset()

	ctx, parent := currentTP.Tracer("test").Start(context.Background(), "request")
	require.NoError(t, mgr.FromContext(ctx).Exec(`INSERT INTO items (name) VALUES ('b')`).Error)
	parent.End()

	dbSpan := spanNamed(t, currentExp.GetSpans(), "gorm.Raw")
	assert.Equal(t, parent.SpanContext().SpanID(), dbSpan.Parent.SpanID())
	assert.Empty(t, retiredExp.GetSpans(), "retired provider must receive nothing")
}

func spanAttr(t *testing.T, span tracetest.SpanStub, key string) string {
	t.Helper()
	for _, kv := range span.Attributes {
		if string(kv.Key) == key {
			return kv.Value.AsString()
		}
	}
	require.FailNowf(t, "attribute not found", "span %q carries no %q", span.Name, key)
	return ""
}

// pinned footgun: upstream's default leaks bound values; that's the app's call
func TestQueryTextIncludesBoundValuesByDefault(t *testing.T) {
	exp, tp := testutil.NewRecorder()
	tr := testutil.NewTracer(tp)
	mgr := setupDB(t, db.WithPlugins(newPlugin(t, tr)))
	exp.Reset()

	require.NoError(t, mgr.FromContext(context.Background()).
		Exec(`INSERT INTO items (name) VALUES (?)`, "hunter2").Error)

	query := spanAttr(t, spanNamed(t, exp.GetSpans(), "gorm.Raw"), "db.query.text")
	assert.Contains(t, query, "INSERT INTO items")
	assert.Contains(t, query, "hunter2")
}

func TestWithoutQueryVariablesExcludesBoundValues(t *testing.T) {
	exp, tp := testutil.NewRecorder()
	tr := testutil.NewTracer(tp)
	mgr := setupDB(t, db.WithPlugins(newPlugin(t, tr, tracing.WithoutQueryVariables())))
	exp.Reset()

	require.NoError(t, mgr.FromContext(context.Background()).
		Exec(`INSERT INTO items (name) VALUES (?)`, "hunter2").Error)

	query := spanAttr(t, spanNamed(t, exp.GetSpans(), "gorm.Raw"), "db.query.text")
	assert.Contains(t, query, "INSERT INTO items")
	assert.NotContains(t, query, "hunter2")
}

func TestCallerOptionsReachThePlugin(t *testing.T) {
	exp, tp := testutil.NewRecorder()
	tr := testutil.NewTracer(tp)
	mgr := setupDB(t, db.WithPlugins(newPlugin(t, tr,
		tracing.WithAttributes(attribute.String("deployment.environment", "test")))))
	exp.Reset()

	require.NoError(t, mgr.FromContext(context.Background()).
		Exec(`INSERT INTO items (name) VALUES ('c')`).Error)

	span := spanNamed(t, exp.GetSpans(), "gorm.Raw")
	assert.Equal(t, "test", spanAttr(t, span, "deployment.environment"))
}

type recordingMeterProvider struct {
	embedded.MeterProvider
	mu     sync.Mutex
	called bool
}

func (p *recordingMeterProvider) Meter(string, ...metric.MeterOption) metric.Meter {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.called = true
	return metricnoop.Meter{}
}

func (p *recordingMeterProvider) wasCalled() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.called
}

func TestMetricsReportingStaysOffTheOtelGlobals(t *testing.T) {
	mp := &recordingMeterProvider{}
	otel.SetMeterProvider(mp)
	// not the previous value: otel rejects re-setting its own delegator
	t.Cleanup(func() { otel.SetMeterProvider(metricnoop.NewMeterProvider()) })

	_, tp := testutil.NewRecorder()
	tr := testutil.NewTracer(tp)
	setupDB(t, db.WithPlugins(newPlugin(t, tr)))

	assert.False(t, mp.wasCalled(), "plugin must not reach for the global MeterProvider")
}

// bare ORM() carries context.Background(), so its spans are roots
func TestQueryViaORMProducesDetachedRootSpan(t *testing.T) {
	exp, tp := testutil.NewRecorder()
	tr := testutil.NewTracer(tp)
	mgr := setupDB(t, db.WithPlugins(newPlugin(t, tr)))
	exp.Reset()

	_, parent := tp.Tracer("test").Start(context.Background(), "request")
	defer parent.End()

	require.NoError(t, mgr.ORM().Exec(`INSERT INTO items (name) VALUES ('c')`).Error)

	dbSpan := spanNamed(t, exp.GetSpans(), "gorm.Raw")
	assert.False(t, dbSpan.Parent.IsValid(), "ORM() carries no context, so its span cannot be parented")
	assert.NotEqual(t, parent.SpanContext().TraceID(), dbSpan.SpanContext.TraceID())
}

func TestQueryViaORMWithContextNestsUnderCallerSpan(t *testing.T) {
	exp, tp := testutil.NewRecorder()
	tr := testutil.NewTracer(tp)
	mgr := setupDB(t, db.WithPlugins(newPlugin(t, tr)))
	exp.Reset()

	ctx, parent := tp.Tracer("test").Start(context.Background(), "request")
	require.NoError(t, mgr.ORM().WithContext(ctx).
		Exec(`INSERT INTO items (name) VALUES ('d')`).Error)
	parent.End()

	dbSpan := spanNamed(t, exp.GetSpans(), "gorm.Raw")
	assert.Equal(t, parent.SpanContext().TraceID(), dbSpan.SpanContext.TraceID())
	assert.Equal(t, parent.SpanContext().SpanID(), dbSpan.Parent.SpanID())
}

func TestNewRejectsNilTracer(t *testing.T) {
	p, err := New(nil)
	require.Error(t, err)
	assert.Nil(t, p, "no plugin alongside an error")
}
