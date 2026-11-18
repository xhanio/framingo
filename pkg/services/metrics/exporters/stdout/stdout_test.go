package stdout_test

import (
	"context"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/xhanio/framingo/pkg/services/metrics"
	"github.com/xhanio/framingo/pkg/utils/confutil"
)

func enabledCtx(config map[string]any) context.Context {
	v := viper.New()
	v.Set("metrics.enabled", true)
	v.Set("metrics.exporter", metrics.Stdout)
	if config != nil {
		v.Set("metrics.config", config)
	}
	return confutil.WrapContext(context.Background(), confutil.New(v))
}

func TestRegistersStdoutExporter(t *testing.T) {
	m := metrics.New()
	require.NoError(t, m.Init(enabledCtx(map[string]any{"pretty": false, "timestamps": false})))
	require.NoError(t, m.Alive(context.Background()))
	require.NoError(t, m.Stop(context.Background()))
}

func TestRejectsUnknownConfigKeys(t *testing.T) {
	m := metrics.New()
	require.Error(t, m.Init(enabledCtx(map[string]any{"endpoint": "stdout has no endpoint"})))
}
