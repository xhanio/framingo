package otlpgrpc_test

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
	v.Set("tracing.exporter", tracing.OTLPGRPC)
	if config != nil {
		v.Set("tracing.config", config)
	}
	return confutil.WrapContext(context.Background(), confutil.New(v))
}

// The otlp exporters connect lazily, so building the provider must succeed
// with no collector listening - a collector outage never fails Init.
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
	// a typo'd compressor must fail Init like the http factory's does, not
	// surface silently at export time
	m := tracing.New()
	err := m.Init(enabledCtx(map[string]any{"compression": "gzp"}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "compression")
}

func TestRejectsUnknownConfigKeys(t *testing.T) {
	// url_path is an http-transport key: the grpc factory's strict decode
	// rejects it instead of silently ignoring it
	m := tracing.New()
	require.Error(t, m.Init(enabledCtx(map[string]any{"url_path": "/v1/traces"})))
}
