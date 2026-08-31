// Package otelgorm installs OpenTelemetry tracing on the db service's GORM
// connection, emitting one client span per statement.
//
// It lives in its own subpackage for the same reason the engine drivers do:
// the upstream plugin imports the ClickHouse, MySQL and PostgreSQL dialectors
// unconditionally (it type-switches on them to set server.address), so pulling
// it into pkg/services/db would link all three client libraries into every
// binary and undo the driver registry's whole purpose. Only a binary that
// wires this plugin pays for it.
//
//	// upstream interpolates bound values into db.query.text; they are where
//	// the credentials and the PII are, and the span goes to a backend less
//	// guarded than the database
//	p, err := otelgorm.New(m.tracing, tracing.WithoutQueryVariables())
//	if err != nil {
//	    return errors.Wrap(err)
//	}
//	m.db = db.New(
//	    db.WithType(config.GetString("db.type")),
//	    db.WithPlugins(p),
//	)
//
// The only default New forces is WithoutMetrics: that half of the plugin
// resolves otel.GetMeterProvider(), the global framingo never touches, and
// upstream offers no way to redirect it. Everything else is the caller's.
package otelgorm

import (
	"github.com/xhanio/errors"
	"gorm.io/gorm"
	"gorm.io/plugin/opentelemetry/tracing"

	"github.com/xhanio/framingo/pkg/types/model"
	"github.com/xhanio/framingo/pkg/utils/traceutil"
)

// New returns a GORM plugin that traces every statement through t. It fails on
// a nil tracer: wiring the plugin before the tracing service is assigned would
// otherwise surface as a nil dereference on the first query, in production and
// far from the cause.
func New(t model.Tracer, opts ...tracing.Option) (gorm.Plugin, error) {
	if t == nil {
		return nil, errors.Newf("otelgorm: nil tracer")
	}

	// caller options come last so they win wherever upstream offers a way to
	// reverse a default. It offers none for WithoutMetrics, which is what
	// makes that one policy rather than a default. WithTracerProvider is
	// reversible, and overriding it gives up the live handle.
	return tracing.NewPlugin(append([]tracing.Option{
		tracing.WithTracerProvider(traceutil.LiveProvider(t)),
		// the plugin's DBStats reporting resolves otel.GetMeterProvider() -
		// the global framingo never touches. There is no metrics service to
		// hand it an explicit handle instead, so that half stays off.
		tracing.WithoutMetrics(),
	}, opts...)...), nil
}
