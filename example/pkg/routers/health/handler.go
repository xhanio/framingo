package health

import (
	"net/http"
	"sort"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/types/entity"

	"github.com/xhanio/framingo/example/pkg/types/api"
)

// Healthz is the process-liveness probe: answering at all is the check.
// Per-service liveness belongs to the supervisor's restart policy - a pod
// restart layered on top would fight a recovery already in progress.
func (r *router) Healthz(c api.Context) error {
	return c.String(http.StatusOK, "ok")
}

// Readyz reports whether every supervised service is ready to serve; 503
// tells load balancers and kubelet to stop routing traffic here while the
// supervisor's monitor works the problem.
func (r *router) Readyz(c api.Context) error {
	stats, err := r.sv.Stats()
	if err != nil {
		return errors.Wrap(err)
	}
	notReady := readyReport(stats)
	if len(notReady) > 0 {
		return c.JSON(http.StatusServiceUnavailable, &api.ReadyzResponse{Services: notReady})
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
