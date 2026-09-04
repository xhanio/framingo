package stdout_test

import (
	"context"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/xhanio/framingo/pkg/services/metrics"
	"github.com/xhanio/framingo/pkg/utils/confutil"
)

// enabledCtx drives the whole exporter choice through config, the authority
// Init reloads from: enabling, the registry name, and the factory's block.
func enabledCtx(config map[string]any) context.Context {
	v := viper.New()
	v.Set("metrics.enabled", true)
	v.Set("metrics.exporter", metrics.Stdout)
	if config != nil {
		v.Set("metrics.config", config)
	}
	return confutil.WrapContext(context.Background(), confutil.New(v))
}

// Importing this package (as any test binary of it does) must register the
// stdout exporter, so an enabled manager builds a real export path from it.
// The manager is never Started and nothing is recorded, so Stop's final
// flush finds an empty reader and prints nothing to the test's stdout.
func TestRegistersStdoutExporter(t *testing.T) {
	m := metrics.New()
	require.NoError(t, m.Init(enabledCtx(map[string]any{"pretty": false, "timestamps": false})))
	require.NoError(t, m.Alive(context.Background()))
	require.NoError(t, m.Stop(true))
}

func TestRejectsUnknownConfigKeys(t *testing.T) {
	m := metrics.New()
	require.Error(t, m.Init(enabledCtx(map[string]any{"endpoint": "stdout has no endpoint"})))
}
