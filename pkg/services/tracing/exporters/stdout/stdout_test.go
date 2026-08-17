package stdout_test

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
	v.Set("tracing.exporter", tracing.Stdout)
	if config != nil {
		v.Set("tracing.config", config)
	}
	return confutil.WrapContext(context.Background(), v)
}

// Importing this package (as any test binary of it does) must register the
// stdout exporter, so an enabled manager builds a real provider from it.
func TestRegistersStdoutExporter(t *testing.T) {
	m := tracing.New()
	require.NoError(t, m.Init(enabledCtx(map[string]any{"pretty": false, "timestamps": false})))
	// a real sampled provider records; the span is deliberately never ended
	// so nothing is exported to the test's stdout
	_, span := m.Provider().Tracer("test").Start(context.Background(), "op")
	assert.True(t, span.IsRecording())
	require.NoError(t, m.Stop(true))
}

func TestRejectsUnknownConfigKeys(t *testing.T) {
	m := tracing.New()
	require.Error(t, m.Init(enabledCtx(map[string]any{"endpoint": "stdout has no endpoint"})))
}
