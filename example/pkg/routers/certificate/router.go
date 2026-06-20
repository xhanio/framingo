package certificate

import (
	_ "embed"
	"path"

	"github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"

	"github.com/xhanio/framingo/example/pkg/types/model"
)

var _ api.Router = (*router)(nil)

//go:embed router.yaml
var config []byte

type router struct {
	name string
	log  log.Logger

	cm model.Certificate
}

func New(cm model.Certificate, log log.Logger) api.Router {
	return &router{
		cm:  cm,
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
		r.cm,
	}
}

func (r *router) Config() []byte {
	return config
}
