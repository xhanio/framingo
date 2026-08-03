package health

import (
	"net/http"
	"sort"

	"github.com/xhanio/framingo/pkg/types/entity"

	"github.com/xhanio/framingo/example/pkg/types/api"
)

// Healthz is the process-liveness probe. It follows the supervisor's own
// liveness, which fails only once in-process recovery is spent - a service
// dead with restarts exhausted - exactly when the platform should replace
// the pod. While the supervisor is still working a problem, answering 200
// is the point: a pod restart would fight the recovery in progress.
func (r *router) Healthz(c api.Context) error {
	if err := r.sv.Alive(); err != nil {
		return c.String(http.StatusServiceUnavailable, err.Error())
	}
	return c.String(http.StatusOK, "ok")
}

// Readyz follows the supervisor's readiness roll-up; 503 tells load
// balancers and kubelet to stop routing traffic here while the monitor
// works the problem, with the not-ready services itemized in the body.
func (r *router) Readyz(c api.Context) error {
	if err := r.sv.Ready(); err != nil {
		// Stats' error return restates per-service healthcheck state, not a
		// failure to fetch - the report below already carries the detail.
		stats, _ := r.sv.Stats()
		return c.JSON(http.StatusServiceUnavailable, &api.ReadyzResponse{Services: readyReport(stats)})
	}
	return c.JSON(http.StatusOK, &api.ReadyzResponse{Ready: true})
}

// readyReport lists the services that are not ready, sorted by name, each
// carrying the most specific error the supervisor recorded for it.
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
