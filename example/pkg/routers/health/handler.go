package health

import (
	"net/http"
	"sort"

	"github.com/xhanio/framingo/pkg/types/entity"

	"github.com/xhanio/framingo/example/pkg/types/api"
)

// Healthz fails only once the supervisor's recovery is spent; a restart before that would fight it.
func (r *router) Healthz(c api.Context) error {
	if err := r.sv.Alive(c); err != nil {
		return c.String(http.StatusServiceUnavailable, err.Error())
	}
	return c.String(http.StatusOK, "ok")
}

func (r *router) Readyz(c api.Context) error {
	if err := r.sv.Ready(c); err != nil {
		// Stats' error only restates per-service state, which the report carries
		stats, _ := r.sv.Stats()
		return c.JSON(http.StatusServiceUnavailable, &api.ReadyzResponse{Services: readyReport(stats)})
	}
	return c.JSON(http.StatusOK, &api.ReadyzResponse{Ready: true})
}

func readyReport(stats []*entity.SupervisorStats) []api.ServiceReadiness {
	var out []api.ServiceReadiness
	for _, s := range stats {
		if s == nil || s.Ready {
			continue
		}
		detail := "not ready"
		switch {
		case s.ReadinessErr != nil:
			detail = s.ReadinessErr.Error()
		case s.Healthcheck() != nil:
			detail = s.Healthcheck().Error()
		}
		out = append(out, api.ServiceReadiness{Name: s.Name, Error: detail})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
