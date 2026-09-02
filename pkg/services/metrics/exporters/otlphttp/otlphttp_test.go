package otlphttp_test

import (
	"context"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xhanio/framingo/pkg/services/metrics"
	"github.com/xhanio/framingo/pkg/utils/confutil"
)

func enabledCtx(config map[string]any) context.Context {
	v := viper.New()
	v.Set("metrics.enabled", true)
	v.Set("metrics.exporter", metrics.OTLPHTTP)
	if config != nil {
		v.Set("metrics.config", config)
	}
	return confutil.WrapContext(context.Background(), v)
}

func TestRegistersOTLPHTTPExporter(t *testing.T) {
	m := metrics.New()
	require.NoError(t, m.Init(enabledCtx(map[string]any{
		"endpoint":    "localhost:14318",
		"url_path":    "/v1/metrics",
		"insecure":    true,
		"compression": "gzip",
		"timeout":     "1s",
	})))
	require.NoError(t, m.Alive(context.Background()))
	require.NoError(t, m.Stop(true))
}

func TestRejectsUnknownCompression(t *testing.T) {
	m := metrics.New()
	err := m.Init(enabledCtx(map[string]any{"compression": "brotli"}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "compression")
}
