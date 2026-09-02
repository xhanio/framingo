package otelutil_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xhanio/framingo/pkg/utils/otelutil"
)

func TestDecodeStrictRejectsUnknownKeys(t *testing.T) {
	var cfg otelutil.Config
	require.Error(t, otelutil.Decode([]byte("max_request_size: 1024\n"), &cfg),
		"max_request_size is a span-exporter knob layered on by tracing's model and must fail here")
}

func TestDecodeDurationAndRetry(t *testing.T) {
	var cfg otelutil.Config
	require.NoError(t, otelutil.Decode([]byte("endpoint: c:4317\ntimeout: 7s\nretry:\n  initial_interval: 2s\n"), &cfg))
	assert.Equal(t, "c:4317", cfg.Endpoint)
	assert.Equal(t, 7*time.Second, cfg.Timeout.Std())
	assert.True(t, cfg.Retry.On(), "a present retry block without enabled is on")
	assert.Equal(t, 2*time.Second, cfg.Retry.InitialInterval.Std())
}

func TestRetryNilIsOn(t *testing.T) {
	var cfg otelutil.Config
	require.NoError(t, otelutil.Decode(nil, &cfg))
	assert.True(t, cfg.Retry.On(), "nil-safe: absent retry reads as the default (on)")
}

func TestRetryOnIsNilSafe(t *testing.T) {
	// exported vocabulary must be total: a factory calling cfg.Retry.On()
	// unguarded gets the default (on), not a panic
	var r *otelutil.Retry
	assert.True(t, r.On())
}

func TestRetryOnTriState(t *testing.T) {
	assert.True(t, (&otelutil.Retry{}).On())
	on, off := true, false
	assert.True(t, (&otelutil.Retry{Enabled: &on}).On())
	assert.False(t, (&otelutil.Retry{Enabled: &off}).On())
}
