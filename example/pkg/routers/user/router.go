package user

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

	um model.User
	rm model.Role
	am model.Auth
}

func New(um model.User, rm model.Role, am model.Auth, log log.Logger) api.Router {
	return &router{
		um:  um,
		rm:  rm,
		am:  am,
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
		r.um,
		r.rm,
		r.am,
	}
}

func (r *router) Config() []byte {
	return config
}
