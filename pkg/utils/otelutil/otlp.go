package otelutil

import (
	"bytes"
	"io"
	"time"

	"github.com/xhanio/errors"
	"gopkg.in/yaml.v3"
)

// Duration decodes a Go duration string ("5s", "1m30s") from yaml, which
// yaml.v3 does not do for time.Duration itself.
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

// Retry mirrors the otlp exporters' retry settings (their RetryConfig types
// are per-package aliases of the same shape).
type Retry struct {
	// Enabled defaults to true when the retry block is present - writing
	// intervals is opting in, matching otlp's own retry-on default.
	Enabled         *bool    `yaml:"enabled"`
	InitialInterval Duration `yaml:"initial_interval"`
	MaxInterval     Duration `yaml:"max_interval"`
	MaxElapsedTime  Duration `yaml:"max_elapsed_time"`
}

// On resolves the tri-state Enabled: a present block without the key is on.
// Nil-safe, so an unguarded cfg.Retry.On() reads as the default (on) rather
// than panicking.
func (r *Retry) On() bool {
	return r == nil || r.Enabled == nil || *r.Enabled
}

// Config is the transport-agnostic half of the otlp exporter surface. A zero
// field leaves the exporter's own default in place. Knobs that carry grpc or
// http types (dial options, connections, proxies) are deliberately absent -
// an exporter needing those is an app-registered Factory instead, whose
// closure captures whatever construction requires.
type Config struct {
	Endpoint    string            `yaml:"endpoint"` // host:port, no scheme
	Insecure    bool              `yaml:"insecure"`
	Headers     map[string]string `yaml:"headers"`
	Compression string            `yaml:"compression"` // "gzip", or ""/"none"
	Timeout     Duration          `yaml:"timeout"`
	Retry       *Retry            `yaml:"retry"`
}

// Decode strict-decodes raw into the factory's config struct: unknown keys
// fail, so a typo'd exporter config entry surfaces at Init instead of being
// silently ignored. nil or empty raw leaves the struct zero.
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
