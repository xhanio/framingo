package otelutil

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeStrictRejectsUnknownKeys(t *testing.T) {
	var cfg Config
	require.Error(t, Decode([]byte("max_request_size: 1024\n"), &cfg),
		"max_request_size is a span-exporter knob layered on by tracing's model and must fail here")
}

func TestDecodeDurationAndRetry(t *testing.T) {
	var cfg Config
	require.NoError(t, Decode([]byte("endpoint: c:4317\ntimeout: 7s\nretry:\n  initial_interval: 2s\n"), &cfg))
	assert.Equal(t, "c:4317", cfg.Endpoint)
	assert.Equal(t, 7*time.Second, cfg.Timeout.Std())
	assert.True(t, cfg.Retry.On(), "a present retry block without enabled is on")
	assert.Equal(t, 2*time.Second, cfg.Retry.InitialInterval.Std())
}

func TestRetryNilIsOn(t *testing.T) {
	var cfg Config
	require.NoError(t, Decode(nil, &cfg))
	assert.True(t, cfg.Retry.On(), "nil-safe: absent retry reads as the default (on)")
}

func TestRetryOnIsNilSafe(t *testing.T) {
	var r *Retry
	assert.True(t, r.On())
}

func TestRetryOnTriState(t *testing.T) {
	assert.True(t, (&Retry{}).On())
	on, off := true, false
	assert.True(t, (&Retry{Enabled: &on}).On())
	assert.False(t, (&Retry{Enabled: &off}).On())
}
