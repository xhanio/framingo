package messagebus

import (
	_ "embed"
	"path"

	"github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/model"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"
)

var _ api.Router = (*router)(nil)

//go:embed router.yaml
var config []byte

type router struct {
	name string
	log  log.Logger

	mb model.MessageBus
}

func New(mb model.MessageBus, log log.Logger) api.Router {
	return &router{
		mb:  mb,
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
	return []common.Service{r.mb}
}

func (r *router) Config() []byte {
	return config
}
