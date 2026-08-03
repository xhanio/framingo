package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/log"

	"github.com/xhanio/framingo/example/pkg/types/api"
)

// fakeSupervisor implements health.Supervisor with canned verdicts and
// stats - the router must consult the supervisor, not re-derive health.
type fakeSupervisor struct {
	alive error
	ready error
	stats []*entity.SupervisorStats
}

func (f *fakeSupervisor) Alive() error                              { return f.alive }
func (f *fakeSupervisor) Ready() error                              { return f.ready }
func (f *fakeSupervisor) Stats() ([]*entity.SupervisorStats, error) { return f.stats, nil }

func serve(t *testing.T, sv *fakeSupervisor, handler func(*router) api.HandlerFunc, path string) *httptest.ResponseRecorder {
	t.Helper()
	r := newRouter(sv, log.Default)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	require.NoError(t, api.WrapHandler(handler(r))(e.NewContext(req, rec)))
	return rec
}

func TestHealthzFollowsSupervisorLiveness(t *testing.T) {
	rec := serve(t, &fakeSupervisor{}, func(r *router) api.HandlerFunc { return r.Healthz }, "/healthz")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "ok", rec.Body.String())

	rec = serve(t, &fakeSupervisor{alive: errors.Newf("service omega failed liveness with recovery spent")},
		func(r *router) api.HandlerFunc { return r.Healthz }, "/healthz")
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), "omega")
}

func TestReadyzFollowsSupervisorReadiness(t *testing.T) {
	rec := serve(t, &fakeSupervisor{}, func(r *router) api.HandlerFunc { return r.Readyz }, "/readyz")
	assert.Equal(t, http.StatusOK, rec.Code)

	sv := &fakeSupervisor{
		ready: errors.Newf("service repo not ready"),
		stats: []*entity.SupervisorStats{
			{Name: "db", Ready: true},
			{Name: "repo", Ready: false, ReadinessErr: errors.Newf("database ping failed")},
		},
	}
	rec = serve(t, sv, func(r *router) api.HandlerFunc { return r.Readyz }, "/readyz")
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	var body api.ReadyzResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.False(t, body.Ready)
	require.Len(t, body.Services, 1)
	assert.Equal(t, "repo", body.Services[0].Name)
	assert.Contains(t, body.Services[0].Error, "ping")
}
