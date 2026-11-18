package stdout

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
	v.Set("tracing.exporter", tracing.Stdout)
	if config != nil {
		v.Set("tracing.config", config)
	}
	return confutil.WrapContext(context.Background(), confutil.New(v))
}

func TestRegistersStdoutExporter(t *testing.T) {
	m := tracing.New()
	require.NoError(t, m.Init(enabledCtx(map[string]any{"pretty": false, "timestamps": false})))
	// never ended, so nothing prints to the test's stdout
	_, span := m.Provider().Tracer("test").Start(context.Background(), "op")
	assert.True(t, span.IsRecording())
	require.NoError(t, m.Stop(context.Background()))
}

func TestRejectsUnknownConfigKeys(t *testing.T) {
	m := tracing.New()
	require.Error(t, m.Init(enabledCtx(map[string]any{"endpoint": "stdout has no endpoint"})))
}
