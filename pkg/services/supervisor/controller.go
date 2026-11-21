package supervisor

import (
	"sync"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/structs/graph"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/confutil"
	"github.com/xhanio/framingo/pkg/utils/log"
)

// controller runs the services' lifecycle: the graph they are registered in,
// their stats, and the policies that bound each step. Its files split it by
// what each does: steps.go a step of one service, gates.go what a step waits
// for, passes.go a step of the whole graph, operations.go a service's own
// operations, reconcile.go what a dependent is told.
type controller struct {
	log    log.Logger
	config confutil.Config

	initPolicy  entity.SupervisorInitPolicy
	startPolicy entity.SupervisorStartPolicy
	stopPolicy  entity.SupervisorStopPolicy

	graph    graph.Graph[common.Service]
	services []common.Service // in dependency order once sorted
	stats    *statsStore
	booted   bool // the first Init has run: a later one makes one pass

	// op serializes the lifecycle: the whole graph's passes, an operation on
	// one service by hand or by the monitor, the shutdown. Exported methods
	// take it, unexported ones run under it.
	op sync.Mutex
}

func newController(config confutil.Config) *controller {
	return &controller{
		config: config,
		graph:  graph.New[common.Service](),
		stats:  newStatsStore(),
	}
}

// register refuses a second instance under a taken name (the graph keys on
// names); the same instance again is a no-op. Wiring-time only: c.services is
// read unlocked.
func (c *controller) register(service common.Service) error {
	if holder := c.stats.track(service); holder != nil {
		if holder != service {
			return errors.Conflict.Newf(
				"service name %s is taken by another instance (%T, now %T): give each its own name, such as with WithName",
				service.Name(), holder, service)
		}
	} else if len(c.services) > 0 {
		c.services = append(c.services, service) // registered after the sort: it goes last
	}
	c.graph.Add(service)
	return nil
}

func (c *controller) addDependency(service, dep common.Service) error {
	if err := c.register(dep); err != nil {
		return err
	}
	c.graph.Add(service, dep)
	return nil
}

func (c *controller) topoSort() error {
	if err := c.graph.TopoSort(); err != nil {
		return errors.Wrap(err)
	}
	c.services = c.graph.Nodes()
	return nil
}

func (c *controller) find(name string) common.Service {
	for _, service := range c.services {
		if service.Name() == name {
			return service
		}
	}
	return nil
}
