package example

import (
	_ "embed"
	"path"

	"github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"

	"github.com/xhanio/framingo/example/pkg/services/example"
)

var _ api.Router = (*router)(nil)

//go:embed router.yaml
var config []byte

type router struct {
	name string
	log  log.Logger

	em example.Manager
}

func New(em example.Manager, log log.Logger) api.Router {
	return &router{
		em:  em,
		log: log,
	}
}

func (r *router) Name() string {
	if r.name == "" {
		r.name = path.Join(reflectutil.Locate(r))
	}
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
