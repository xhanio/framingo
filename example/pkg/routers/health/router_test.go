package health

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/log"
	"gopkg.in/yaml.v3"
)

// newRouter hands tests the concrete *router, without the fapi.Router
// interface New wraps it in for everyone else.
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
	// Every func the yaml declares must resolve to a handler, or registration
	// fails at startup - and discovery skips mismatched signatures silently,
	// so this is the only place short of booting the server that catches it.
	for _, h := range cfg.Handlers {
		assert.Contains(t, handlers, h.Func,
			"router.yaml declares func %s but no matching handler method was discovered", h.Func)
	}
}

func TestReadyReport(t *testing.T) {
	// All ready: empty report.
	stats := []*entity.SupervisorStats{
		{Name: "db", Ready: true},
		{Name: "repo", Ready: true},
	}
	notReady := readyReport(stats)
	assert.Empty(t, notReady)

	// One not ready, with its readiness error carried into the report and
	// results sorted by name for stable output.
	stats = []*entity.SupervisorStats{
		{Name: "repo", Ready: false, ReadinessErr: errors.Newf("database ping failed")},
		{Name: "db", Ready: true},
		{Name: "example", Ready: false},
	}
	notReady = readyReport(stats)
	require.Len(t, notReady, 2)
	assert.Equal(t, "example", notReady[0].Name)
	assert.Equal(t, "repo", notReady[1].Name)
	assert.Contains(t, notReady[1].Error, "ping")
	assert.NotEmpty(t, notReady[0].Error, "a not-ready service with no recorded error still explains itself")
}
