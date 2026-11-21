package supervisor

import (
	"context"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
)

// What a step waits for. A service the supervisor does not track passes every
// gate, and a nil dependency — an optional one the app did not wire — is
// skipped.

// initialized: its last Init succeeded.
func (c *controller) initialized(service common.Service) bool {
	stat := c.stats.snapshot(service.Name())
	return stat == nil || stat.Initialized
}

// running: initialized, started, not stopped, and neither its start nor a
// Reconcile since has failed.
func (c *controller) running(service common.Service) bool {
	stat := c.stats.snapshot(service.Name())
	if stat == nil {
		return true
	}
	return stat.Initialized && stat.StartErr == nil && stat.ReconcileErr == nil && !stat.Stopped &&
		(!isDaemon(service) || stat.Started)
}

// depsRunning is the first dependency not running: what gates a start, and a
// restart.
func (c *controller) depsRunning(service common.Service) error {
	for _, dep := range service.Dependencies() {
		if dep != nil && !c.running(dep) {
			return errors.Unavailable.Newf("dependency %s is not running", dep.Name())
		}
	}
	return nil
}

// depsInitialized is what gates an init turn. A dependency whose Init failed
// fails it for good; under a policy that waits, a dependency whose Ready
// probe fails holds it for another try.
func (c *controller) depsInitialized(ctx context.Context, service common.Service, policy entity.SupervisorInitPolicy) (blocked error, transient bool) {
	for _, dep := range service.Dependencies() {
		if dep == nil {
			continue
		}
		if !c.initialized(dep) {
			c.log.Debugf("%s dependency %s is not initialized", service.Name(), dep.Name())
			return errors.Newf("dependencies not ready"), false
		}
		if policy.MaxRetries == 0 {
			continue
		}
		if readiness, ok := dep.(common.Readiness); ok {
			if err := readiness.Ready(ctx); err != nil {
				return errors.Wrapf(err, "dependency %s not ready", dep.Name()), true
			}
		}
	}
	return nil, false
}

// dependsOn by name, as the graph keys: one instance per name.
func dependsOn(service, dep common.Service) bool {
	for _, d := range service.Dependencies() {
		if d != nil && d.Name() == dep.Name() {
			return true
		}
	}
	return false
}
