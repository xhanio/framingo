package user

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhanio/framingo/pkg/utils/log"
	"gopkg.in/yaml.v3"
)

// newRouter hands tests the concrete *router, without the fapi.Router
// interface New wraps it in for everyone else.
func TestHandlersCoverConfig(t *testing.T) {
	r := newRouter(nil, nil, nil, log.Default)
	handlers := r.Handlers()
	require.NotEmpty(t, handlers)

	var cfg struct {
		Handlers []struct {
			Func string `yaml:"func"`
		} `yaml:"handlers"`
	}
	require.NoError(t, yaml.Unmarshal(r.Config(), &cfg))
	require.NotEmpty(t, cfg.Handlers)
	// Every func the yaml declares must resolve to a handler, or registration
	// fails at startup - and discovery skips mismatched signatures silently,
	// so this is the only place short of booting the server that catches it.
	for _, h := range cfg.Handlers {
		assert.Contains(t, handlers, h.Func,
			"router.yaml declares func %s but no matching handler method was discovered", h.Func)
	}
}
