// Package otelutil holds the shared plumbing behind framingo's telemetry
// services (tracing, metrics): the name-to-factory Registry their exporter
// subpackages register into from init(), and the transport-agnostic otlp
// config vocabulary (Duration, Retry, Config, the strict Decode) their
// factories build exporters from. Signal-specific knobs layer on top in the
// signal's own package - tracing's exporters/model wraps Config with the
// span-exporter-only max_request_size, which is why it is absent here.
package otelutil
