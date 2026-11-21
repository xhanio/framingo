package health

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/log"

	"github.com/xhanio/framingo/example/pkg/utils/apitest"
)

func TestRouter(t *testing.T) {
	apitest.CheckRouter(t, newRouter(nil, log.Default))
}

func TestReadyReport(t *testing.T) {
	stats := []*entity.SupervisorStats{
		{Name: "db", Ready: true},
		{Name: "repo", Ready: true},
	}
	notReady := readyReport(stats)
	assert.Empty(t, notReady)

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
