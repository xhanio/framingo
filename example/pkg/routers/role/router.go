package role

import (
	_ "embed"
	"path"

	fapi "github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"

	"github.com/xhanio/framingo/example/pkg/types/api"
	"github.com/xhanio/framingo/example/pkg/types/model"
)

var _ fapi.Router = (*router)(nil)

//go:embed router.yaml
var config []byte

type router struct {
	name string
	log  log.Logger

	rm model.Role
}

func New(rm model.Role, log log.Logger) fapi.Router {
	return newRouter(rm, log)
}

// newRouter returns the concrete router, the form package tests construct.
func newRouter(rm model.Role, log log.Logger) *router {
	r := &router{
		rm:  rm,
		log: log,
	}
	if r.name == "" {
		r.name = path.Join(reflectutil.Locate(r))
	}
	return r
}

func (r *router) Name() string {
	return r.name
}

func (r *router) Dependencies() []common.Service {
	return []common.Service{
		r.rm,
	}
}

func (r *router) Config() []byte {
	return config
}

func (r *router) Handlers() map[string]any {
	handlers := api.DiscoverHandlers(r)
	r.log.Debugf("router %s parsed %d handler(s)", r.Name(), len(handlers))
	return handlers
}
