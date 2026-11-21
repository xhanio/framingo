package supervisor

import (
	"context"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
)

// stateAfter is the state a start's err leaves its service in.
func stateAfter(err error) common.DependencyState {
	if err != nil {
		return common.DependencyFailed
	}
	return common.DependencyRunning
}

// reconcile tells each running dependent of dep that is Reconcilable what dep
// has become, in dependency order, each call bounded by the start policy. A
// dependent that fails is failed, its first failure kept, for the monitor to
// restart.
func (c *controller) reconcile(ctx context.Context, dep common.Service, state common.DependencyState) {
	for _, service := range c.services {
		svc, ok := service.(common.Reconcilable)
		if !ok || service.Name() == dep.Name() || !dependsOn(service, dep) || !c.running(service) {
			continue
		}
		rctx, cancel := c.startContext(ctx)
		err := svc.Reconcile(rctx, dep, state)
		cancel()
		if err == nil {
			continue
		}
		err = errors.Wrapf(err, "failed to reconcile with %s %s", dep.Name(), state)
		c.log.Errorf("service %s: %s", service.Name(), err)
		c.stats.update(service.Name(), func(stat *entity.SupervisorStats) {
			if stat.ReconcileErr == nil {
				stat.ReconcileErr = err
			}
			stat.Ready = false
		})
	}
}
