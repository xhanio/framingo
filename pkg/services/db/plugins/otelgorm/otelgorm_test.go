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

// newPlugin fails the test rather than threading the error through every case;
// the nil-tracer path has its own test.
func newPlugin(t *testing.T, tr model.Tracer, opts ...tracing.Option) gorm.Plugin {
	t.Helper()
	p, err := New(tr, opts...)
	require.NoError(t, err)
	return p
}

// setupDB builds a SQLite-backed manager with the given options. max_open=1
// keeps every query on the one :memory: connection, so the table created here
// stays visible to the queries under test.
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

// The whole point of instrumenting the db service: a query issued through
// FromContext must land inside the caller's trace, not start one of its own.
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

// A tracing-service restart swaps the provider behind model.Tracer, and the
// upstream plugin resolves its tracer once, at construction. So the handle it
// captures has to stay live - otherwise db spans keep going to the retired
// provider and silently vanish, with nothing failing to say so.
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

// Upstream interpolates bound values into db.query.text by default and New
// passes that through: whether query variables are acceptable depends on the
// data and the backend, which is the app's call, not framingo's. Pinned
// because it is a footgun - the values are exactly where the credentials and
// the PII sit, and a span travels somewhere far less guarded than the database.
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

// The opt-out is the wiring any app handling real data should write, so it is
// pinned end-to-end through New rather than trusted to the generic
// pass-through test. Characterization, not a TDD cycle: option pass-through is
// already proven by TestCallerOptionsReachThePlugin, so this never could fail.
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

// The point of taking upstream's own option type rather than wrapping it:
// whatever the caller passes has to actually reach the plugin, instead of
// sitting in a wrapper that silently drops the ones it never learned about.
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

// recordingMeterProvider records whether anything asked it for a Meter.
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

// The upstream plugin also reports DBStats metrics, and that path reaches for
// otel.GetMeterProvider() - the global. Framingo hands every component an
// explicit handle instead and never touches the otel globals, so that half of
// the plugin has to stay switched off.
func TestMetricsReportingStaysOffTheOtelGlobals(t *testing.T) {
	mp := &recordingMeterProvider{}
	otel.SetMeterProvider(mp)
	// restoring the previous value would hand the global delegator back to
	// itself, which otel refuses with an error on stderr - a fresh noop is
	// the same effective default, quietly
	t.Cleanup(func() { otel.SetMeterProvider(metricnoop.NewMeterProvider()) })

	_, tp := testutil.NewRecorder()
	tr := testutil.NewTracer(tp)
	setupDB(t, db.WithPlugins(newPlugin(t, tr)))

	assert.False(t, mp.wasCalled(), "plugin must not reach for the global MeterProvider")
}

// Bare ORM() hands back the base *gorm.DB, whose Statement.Context is
// context.Background() - there is no caller in it to be part of. Queries
// issued that way are still traced, but as detached root spans: real timings,
// no link to the request that caused them. It is the missing context that
// breaks the link, not ORM() itself - see the WithContext case below.
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

// The plugin parents off tx.Statement.Context, and WithContext is what sets
// it - so ORM().WithContext(ctx) nests exactly like FromContext(ctx), whose
// non-transaction branch is literally that call. Pinned because the two are
// easy to describe as opposites when the real distinction is whether a context
// reached the statement. (Note ORM().WithContext bypasses the transaction a
// context may carry, which is FromContext's other job - a correctness matter,
// not a tracing one.) Characterization: gorm and the upstream plugin already
// behave this way, so this never could have failed.
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

// Wiring the plugin before the tracing service is assigned is a programmer
// error whose natural failure - a nil dereference on the first query - lands
// far from the cause, in production. Reporting it at wiring time puts it in
// front of Init, which is where the app can still do something about it.
func TestNewRejectsNilTracer(t *testing.T) {
	p, err := New(nil)
	require.Error(t, err)
	assert.Nil(t, p, "no plugin alongside an error")
}
