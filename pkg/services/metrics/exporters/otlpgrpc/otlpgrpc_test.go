package otlpgrpc_test

import (
	"context"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xhanio/framingo/pkg/services/metrics"
	"github.com/xhanio/framingo/pkg/utils/confutil"
)

// enabledCtx drives the whole exporter choice through config, the authority
// Init reloads from: enabling, the registry name, and the factory's block.
func enabledCtx(config map[string]any) context.Context {
	v := viper.New()
	v.Set("metrics.enabled", true)
	v.Set("metrics.exporter", metrics.OTLPGRPC)
	if config != nil {
		v.Set("metrics.config", config)
	}
	return confutil.WrapContext(context.Background(), confutil.New(v))
}

// The otlp exporters connect lazily, so building the export path must
// succeed with no collector listening - a collector outage never fails
// Init. Nothing is recorded, so Stop's final flush has nothing to send.
func TestRegistersOTLPGRPCExporter(t *testing.T) {
	m := metrics.New()
	require.NoError(t, m.Init(enabledCtx(map[string]any{
		"endpoint":    "localhost:14317",
		"insecure":    true,
		"headers":     map[string]any{"authorization": "Bearer x"},
		"compression": "gzip",
		"timeout":     "1s",
		"retry":       map[string]any{"initial_interval": "2s", "max_interval": "10s"},
	})))
	require.NoError(t, m.Alive(context.Background()))
	require.NoError(t, m.Stop(context.Background()))
}

func TestRejectsUnknownCompression(t *testing.T) {
	// a typo'd compressor must fail Init like the http factory's does, not
	// surface silently at export time
	m := metrics.New()
	err := m.Init(enabledCtx(map[string]any{"compression": "gzp"}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "compression")
}

func TestRejectsUnknownConfigKeys(t *testing.T) {
	// url_path is an http-transport key: the grpc factory's strict decode
	// rejects it instead of silently ignoring it
	m := metrics.New()
	require.Error(t, m.Init(enabledCtx(map[string]any{"url_path": "/v1/metrics"})))
}
