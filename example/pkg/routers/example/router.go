package example

import (
	_ "embed"

	fapi "github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/nameutil"

	"github.com/xhanio/framingo/example/pkg/types/api"
	"github.com/xhanio/framingo/example/pkg/types/model"
)

var _ fapi.Router = (*router)(nil)

//go:embed router.yaml
var config []byte

type router struct {
	name string
	log  log.Logger

	em model.Example
}

func New(em model.Example, log log.Logger) fapi.Router {
	return newRouter(em, log)
}

// newRouter returns the concrete router, the form package tests construct.
func newRouter(em model.Example, log log.Logger) *router {
	r := &router{
		em:  em,
		log: log,
	}
	r.name = nameutil.Name(r)
	return r
}

func (r *router) Name() string {
	return r.name
}

func (r *router) Dependencies() []common.Service {
	return []common.Service{
		r.em,
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
