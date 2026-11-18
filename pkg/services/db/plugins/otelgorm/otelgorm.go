// Package otelgorm traces GORM statements; a separate package because upstream
// imports the clickhouse, mysql and postgres dialectors.
//
//	// drop bound values (credentials, PII) from db.query.text
//	p, err := otelgorm.New(m.tracing, tracing.WithoutQueryVariables())
//	if err != nil {
//	    return errors.Wrap(err)
//	}
//	m.db = db.New(
//	    db.WithType(config.GetString("db.type")),
//	    db.WithPlugins(p),
//	)
package otelgorm

import (
	"github.com/xhanio/errors"
	"gorm.io/gorm"
	"gorm.io/plugin/opentelemetry/tracing"

	"github.com/xhanio/framingo/pkg/types/model"
	"github.com/xhanio/framingo/pkg/utils/traceutil"
)

// New fails on a nil tracer rather than panic on the first query.
func New(t model.Tracer, opts ...tracing.Option) (gorm.Plugin, error) {
	if t == nil {
		return nil, errors.Newf("otelgorm: nil tracer")
	}

	// caller options last so they win; overriding WithTracerProvider loses the live handle
	return tracing.NewPlugin(append([]tracing.Option{
		tracing.WithTracerProvider(traceutil.LiveProvider(t)),
		// DBStats metrics would use otel.GetMeterProvider(), the global; no way to redirect it
		tracing.WithoutMetrics(),
	}, opts...)...), nil
}
