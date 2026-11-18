package otelutil

import (
	"bytes"
	"io"
	"time"

	"github.com/xhanio/errors"
	"gopkg.in/yaml.v3"
)

// Duration decodes "5s"-style yaml, which yaml.v3 does not do for time.Duration.
type Duration time.Duration

func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	var s string
	if err := node.Decode(&s); err != nil {
		return errors.Wrap(err)
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return errors.Wrap(err)
	}
	*d = Duration(v)
	return nil
}

func (d Duration) Std() time.Duration {
	return time.Duration(d)
}

type Retry struct {
	// nil means on, matching otlp's default
	Enabled         *bool    `yaml:"enabled"`
	InitialInterval Duration `yaml:"initial_interval"`
	MaxInterval     Duration `yaml:"max_interval"`
	MaxElapsedTime  Duration `yaml:"max_elapsed_time"`
}

// On is nil-safe: a nil Retry or unset Enabled is on.
func (r *Retry) On() bool {
	return r == nil || r.Enabled == nil || *r.Enabled
}

// Config is the transport-agnostic otlp config; a zero field keeps the exporter's default.
// grpc/http-typed knobs belong in an app-registered Factory instead.
type Config struct {
	Endpoint    string            `yaml:"endpoint"` // host:port, no scheme
	Insecure    bool              `yaml:"insecure"`
	Headers     map[string]string `yaml:"headers"`
	Compression string            `yaml:"compression"` // "gzip", or ""/"none"
	Timeout     Duration          `yaml:"timeout"`
	Retry       *Retry            `yaml:"retry"`
}

// Decode rejects unknown keys; empty raw leaves into untouched.
func Decode(raw []byte, into any) error {
	if len(raw) == 0 {
		return nil
	}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(into); err != nil && !errors.Is(err, io.EOF) {
		return errors.Wrap(err)
	}
	return nil
}
