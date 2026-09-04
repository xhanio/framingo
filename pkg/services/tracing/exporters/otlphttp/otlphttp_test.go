package otlphttp_test

import (
	"context"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xhanio/framingo/pkg/services/tracing"
	"github.com/xhanio/framingo/pkg/utils/confutil"
)

// enabledCtx drives the whole exporter choice through config, the authority
// Init reloads from: enabling, the registry name, and the factory's block.
func enabledCtx(config map[string]any) context.Context {
	v := viper.New()
	v.Set("tracing.enabled", true)
	v.Set("tracing.exporter", tracing.OTLPHTTP)
	if config != nil {
		v.Set("tracing.config", config)
	}
	return confutil.WrapContext(context.Background(), confutil.New(v))
}

// The otlp exporters connect lazily, so building the provider must succeed
// with no collector listening - a collector outage never fails Init.
func TestRegistersOTLPHTTPExporter(t *testing.T) {
	m := tracing.New()
	require.NoError(t, m.Init(enabledCtx(map[string]any{
		"endpoint":    "localhost:14318",
		"url_path":    "/custom/v1/traces",
		"insecure":    true,
		"compression": "gzip",
		"timeout":     "1s",
		"retry":       map[string]any{"enabled": false},
	})))
	_, span := m.Provider().Tracer("test").Start(context.Background(), "op")
	assert.True(t, span.IsRecording())
	require.NoError(t, m.Stop(true))
}

func TestRejectsUnknownConfigKeys(t *testing.T) {
	m := tracing.New()
	require.Error(t, m.Init(enabledCtx(map[string]any{"endponit": "typo:4318"})))
}

func TestRejectsUnknownCompression(t *testing.T) {
	m := tracing.New()
	err := m.Init(enabledCtx(map[string]any{"compression": "zstd"}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "compression")
}
