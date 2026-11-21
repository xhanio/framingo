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
	"github.com/xhanio/framingo/pkg/utils/printutil"
)

func (m *manager) Init(ctx context.Context) error {
	s := m.resolve(confutil.FromContext(ctx))
	// published whole: probes and Info read the settings concurrently
	m.mu.Lock()
	m.settings = s
	m.mu.Unlock()
	enabled := s.enabled
	// a failed flush loses buffered spans, never the restart
	if err := m.controller.flush(context.Background(), m.controller.retire(false)); err != nil {
		m.log.Warnf("failed to flush retiring tracer provider: %s", err)
	}
	if !enabled {
		return nil
	}
	exp, err := m.buildExporter(ctx, s)
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
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(s.sampleRatio))),
		sdktrace.WithBatcher(exp),
	)
	m.controller.promote(tp)
	return nil
}

// buildExporter builds the exporter s names.
func (m *manager) buildExporter(ctx context.Context, s settings) (sdktrace.SpanExporter, error) {
	if m.exporter != nil {
		return m.exporter, nil
	}
	if s.exporterName == "" {
		return nil, errors.Newf("tracing %s is enabled but no exporter is configured", m.Name())
	}
	f, err := lookupFactory(s.exporterName)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	var raw []byte
	if len(s.exporterConfig) > 0 {
		if raw, err = yaml.Marshal(s.exporterConfig); err != nil {
			return nil, errors.Wrapf(err, "invalid tracing exporter config")
		}
	}
	// otlp connects lazily, so an unreachable collector does not fail the build
	exp, err := f(ctx, raw)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to build %s span exporter", s.exporterName)
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
	s := m.settings
	m.mu.RUnlock()
	enabled, exporter, sampleRatio := s.enabled, s.exporterName, s.sampleRatio
	if m.exporter != nil {
		exporter = "custom"
	}
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
