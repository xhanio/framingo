package user

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/xhanio/framingo/pkg/utils/log"
)

// newRouter hands tests the concrete *router, without the fapi.Router
// interface New wraps it in for everyone else.
func TestHandlersCoverConfig(t *testing.T) {
	r := newRouter(nil, nil, log.Default)
	handlers := r.Handlers()
	require.NotEmpty(t, handlers)

	// router.yaml holds two handler groups, one per document: read them all,
	// not just the first.
	dec := yaml.NewDecoder(bytes.NewReader(r.Config()))
	groups := 0
	for {
		var cfg struct {
			Handlers []struct {
				Func string `yaml:"func"`
			} `yaml:"handlers"`
		}
		err := dec.Decode(&cfg)
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		require.NotEmpty(t, cfg.Handlers)
		groups++
		// Every func the yaml declares must resolve to a handler, or
		// registration fails at startup - and discovery skips mismatched
		// signatures silently, so this is the only place short of booting
		// the server that catches it.
		for _, h := range cfg.Handlers {
			assert.Contains(t, handlers, h.Func,
				"router.yaml declares func %s but no matching handler method was discovered", h.Func)
		}
	}
	assert.Equal(t, 2, groups, "the /users and /user groups")
}
