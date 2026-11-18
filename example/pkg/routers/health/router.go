package health

import (
	_ "embed"

	fapi "github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/nameutil"

	"github.com/xhanio/framingo/example/pkg/types/api"
)

var _ fapi.Router = (*router)(nil)

// Supervisor is narrow so the service package stays out of the router; supervisor.Manager satisfies it.
type Supervisor interface {
	common.Liveness
	common.Readiness
	Stats() ([]*entity.SupervisorStats, error)
}

//go:embed router.yaml
var config []byte

type router struct {
	name string
	log  log.Logger

	sv Supervisor
}

func New(sv Supervisor, log log.Logger) fapi.Router {
	return newRouter(sv, log)
}

func newRouter(sv Supervisor, log log.Logger) *router {
	r := &router{
		sv:  sv,
		log: log,
	}
	r.name = nameutil.Name(r)
	return r
}

func (r *router) Name() string {
	return r.name
}

// Dependencies is empty: the supervisor orchestrates the graph and cannot be a node in it.
func (r *router) Dependencies() []common.Service {
	return nil
}

func (r *router) Config() []byte {
	return config
}

func (r *router) Handlers() map[string]any {
	handlers := api.DiscoverHandlers(r)
	r.log.Debugf("router %s parsed %d handler(s)", r.Name(), len(handlers))
	return handlers
}
