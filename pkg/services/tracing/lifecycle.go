package tracing

import (
	"context"
	"io"

	"github.com/xhanio/errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"gopkg.in/yaml.v3"

	"github.com/xhanio/framingo/pkg/types/info"
	"github.com/xhanio/framingo/pkg/utils/confutil"
	"github.com/xhanio/framingo/pkg/utils/nameutil"
	"github.com/xhanio/framingo/pkg/utils/printutil"
)

func (m *manager) Init(ctx context.Context) error {
	config := confutil.FromContext(ctx)
	root := nameutil.ConfigRoot(m.name)
	// Dynamic config reloads through the very options the wiring layer used,
	// so a value has one way in and setDefaults has the only say on what a
	// zero falls back to - the constructor's order, repeated. Config is the
	// authority here: an absent key reads zero and lands on the default
	// rather than preserving whatever construction happened to set. Under
	// the lock because probes and Info read these fields concurrently from
	// the monitor's and the supervisor's goroutines. The exporter block
	// stays opaque - the chosen factory strict-decodes its own struct.
	m.mu.Lock()
	m.applyOptions(
		WithEnabled(config.GetBool("%s.enabled", root)),
		WithExporter(
			config.GetString("%s.exporter", root),
			config.GetStringMap("%s.config", root),
		),
		WithSampleRatio(config.GetFloat64("%s.sample_ratio", root)),
	)
	m.setDefaults()
	enabled := m.enabled
	m.mu.Unlock()
	// Init runs on first startup and on every restart: retire and flush the
	// previous pipeline before building its replacement. A failed flush only
	// loses buffered spans - never the restart.
	if err := m.controller.flush(m.controller.retire(false)); err != nil {
		m.log.Warnf("failed to flush retiring tracer provider: %s", err)
	}
	if !enabled {
		return nil
	}
	exp, err := m.buildExporter(ctx)
	if err != nil {
		return err
	}
	// The span resource is the build's application identity, nothing else:
	// the binary knows which application it is, so neither attribute is
	// configurable. A build without ldflags injection leaves them empty and
	// the attribute is omitted rather than set blank, so a backend shows its
	// own unknown-service placeholder instead of a nameless service.
	var attrs []attribute.KeyValue
	if info.ApplicationName != "" {
		attrs = append(attrs, semconv.ServiceName(info.ApplicationName))
	}
	if info.ApplicationVersion != "" {
		attrs = append(attrs, semconv.ServiceVersion(info.ApplicationVersion))
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(resource.NewWithAttributes(semconv.SchemaURL, attrs...)),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(m.sampleRatio))),
		sdktrace.WithBatcher(exp),
	)
	m.controller.promote(tp)
	return nil
}

// buildExporter resolves where spans go: the injected instance when the
// test seam is set, else the registry factory named by exporterName fed the
// marshaled config block. It reads the config fields unlocked - Init, the
// only writer, already rewrote them earlier in the same call, and
// supervisor ops serialize.
func (m *manager) buildExporter(ctx context.Context) (sdktrace.SpanExporter, error) {
	if m.exporter != nil {
		return m.exporter, nil
	}
	if m.exporterName == "" {
		return nil, errors.Newf("tracing %s is enabled but no exporter is configured", m.Name())
	}
	f, err := lookupFactory(m.exporterName)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	var raw []byte
	if len(m.exporterConfig) > 0 {
		if raw, err = yaml.Marshal(m.exporterConfig); err != nil {
			return nil, errors.Wrapf(err, "invalid tracing exporter config")
		}
	}
	// otlp exporters connect lazily, so an unreachable collector does not
	// fail the build - matching the probe doctrine that a collector outage
	// must never take the app down
	exp, err := f(ctx, raw)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to build %s span exporter", m.exporterName)
	}
	return exp, nil
}

// Start is a no-op: the span processor runs its own export loop. Daemon is
// implemented so the supervisor calls Stop on shutdown and restart, which is
// where the final flush happens.
func (m *manager) Start(_ context.Context) error {
	return nil
}

// Stop flushes and retires the provider. The flush is synchronous either way
// - bounded by shutdownTimeout - so wait needs no separate slow path.
func (m *manager) Stop(_ bool) error {
	return m.controller.flush(m.controller.retire(true))
}

func (m *manager) Info(w io.Writer, debug bool) {
	m.mu.RLock()
	enabled := m.enabled
	exporter := m.exporterName
	if m.exporter != nil {
		exporter = "custom"
	}
	sampleRatio := m.sampleRatio
	m.mu.RUnlock()
	state := "noop"
	if live, _ := m.controller.state(); live {
		state = "active"
	}
	t := printutil.NewTable(w)
	t.Header(m.Name())
	t.Title("stat", "value")
	t.Row("enabled", enabled)
	t.Row("exporter", exporter)
	t.Row("provider", state)
	if debug {
		t.Row("sample_ratio", sampleRatio)
		t.Row("service_name", info.ApplicationName)
	}
	t.NewLine()
	t.Flush()
}
