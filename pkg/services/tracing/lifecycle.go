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
	// reload through the constructor's options so setDefaults decides every zero;
	// locked: probes and Info read these fields concurrently
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
	// a failed flush loses buffered spans, never the restart
	if err := m.controller.flush(context.Background(), m.controller.retire(false)); err != nil {
		m.log.Warnf("failed to flush retiring tracer provider: %s", err)
	}
	if !enabled {
		return nil
	}
	exp, err := m.buildExporter(ctx)
	if err != nil {
		return err
	}
	// omit unset attributes rather than blank them, so backends show their placeholder
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

// buildExporter reads config unlocked: Init is the only writer and calls it.
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
	// otlp connects lazily, so an unreachable collector does not fail the build
	exp, err := f(ctx, raw)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to build %s span exporter", m.exporterName)
	}
	return exp, nil
}

// Start is a no-op; Daemon exists so the supervisor calls Stop to flush.
func (m *manager) Start(_ context.Context) error {
	return nil
}

func (m *manager) Stop(ctx context.Context) error {
	return m.controller.flush(ctx, m.controller.retire(true))
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
