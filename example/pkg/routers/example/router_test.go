package example

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/xhanio/framingo/pkg/utils/log"
)

func TestHandlersCoverConfig(t *testing.T) {
	r := newRouter(nil, log.Default)
	handlers := r.Handlers()
	require.NotEmpty(t, handlers)

	var cfg struct {
		Handlers []struct {
			Func string `yaml:"func"`
		} `yaml:"handlers"`
	}
	require.NoError(t, yaml.Unmarshal(r.Config(), &cfg))
	require.NotEmpty(t, cfg.Handlers)
	// discovery skips mismatched signatures silently; only this catches them before boot
	for _, h := range cfg.Handlers {
		assert.Contains(t, handlers, h.Func,
			"router.yaml declares func %s but no matching handler method was discovered", h.Func)
	}
}
