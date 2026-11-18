package otlpgrpc

import (
	"context"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xhanio/framingo/pkg/services/tracing"
	"github.com/xhanio/framingo/pkg/utils/confutil"
)

func enabledCtx(config map[string]any) context.Context {
	v := viper.New()
	v.Set("tracing.enabled", true)
	v.Set("tracing.exporter", tracing.OTLPGRPC)
	if config != nil {
		v.Set("tracing.config", config)
	}
	return confutil.WrapContext(context.Background(), confutil.New(v))
}

// no collector is listening: otlp connects lazily
func TestRegistersOTLPGRPCExporter(t *testing.T) {
	m := tracing.New()
	require.NoError(t, m.Init(enabledCtx(map[string]any{
		"endpoint":         "localhost:14317",
		"insecure":         true,
		"headers":          map[string]any{"authorization": "Bearer x"},
		"compression":      "gzip",
		"timeout":          "1s",
		"max_request_size": 1 << 20,
		"retry":            map[string]any{"initial_interval": "2s", "max_interval": "10s"},
	})))
	_, span := m.Provider().Tracer("test").Start(context.Background(), "op")
	assert.True(t, span.IsRecording())
	require.NoError(t, m.Stop(context.Background()))
}

func TestRejectsUnknownCompression(t *testing.T) {
	m := tracing.New()
	err := m.Init(enabledCtx(map[string]any{"compression": "gzp"}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "compression")
}

func TestRejectsUnknownConfigKeys(t *testing.T) {
	// url_path is http-only
	m := tracing.New()
	require.Error(t, m.Init(enabledCtx(map[string]any{"url_path": "/v1/traces"})))
}
