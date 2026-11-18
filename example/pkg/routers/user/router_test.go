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

func TestHandlersCoverConfig(t *testing.T) {
	r := newRouter(nil, nil, log.Default)
	handlers := r.Handlers()
	require.NotEmpty(t, handlers)

	// one handler group per yaml document
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
		// discovery skips mismatched signatures silently; only this catches them before boot
		for _, h := range cfg.Handlers {
			assert.Contains(t, handlers, h.Func,
				"router.yaml declares func %s but no matching handler method was discovered", h.Func)
		}
	}
	assert.Equal(t, 2, groups, "the /users and /user groups")
}
