package example

import (
	"context"
	_ "net/http/pprof"
	"path"

	"github.com/spf13/viper"
	"github.com/xhanio/framingo/pkg/services/api/server"
	"github.com/xhanio/framingo/pkg/services/db"
	"github.com/xhanio/framingo/pkg/services/pubsub"
	"github.com/xhanio/framingo/pkg/services/supervisor"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"

	"github.com/xhanio/framingo/example/pkg/services/example"
	"github.com/xhanio/framingo/example/pkg/services/repository"
	"github.com/xhanio/framingo/example/pkg/services/system/auth"
	"github.com/xhanio/framingo/example/pkg/services/system/certificate"
	"github.com/xhanio/framingo/example/pkg/services/system/organization"
	"github.com/xhanio/framingo/example/pkg/services/system/role"
	"github.com/xhanio/framingo/example/pkg/services/system/user"
)

type manager struct {
	name   string
	config *viper.Viper
	// util services
	log log.Logger

	// infra services
	db         db.Manager
	bus        pubsub.Manager
	repository repository.Repository

	// system services
	user         user.Manager
	role         role.Manager
	organization organization.Manager
	certificate  certificate.Manager
	auth         auth.Manager

	// business services
	example example.Manager

	// api related services
	api server.Manager

	// service controller
	services supervisor.Manager

	// shutdown control
	ctx    context.Context
	cancel context.CancelFunc
}

func New(configPath string) Server {
	return &manager{
		config: newConfig(configPath),
	}
}

func (m *manager) Name() string {
	if m.name == "" {
		m.name = path.Join(reflectutil.Locate(m))
	}
	return m.name
}
