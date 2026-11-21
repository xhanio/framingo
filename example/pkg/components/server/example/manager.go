package example

import (
	"context"
	_ "net/http/pprof"

	"github.com/spf13/viper"

	"github.com/xhanio/framingo/pkg/services/api/server"
	"github.com/xhanio/framingo/pkg/services/db"
	"github.com/xhanio/framingo/pkg/services/messagebus"
	"github.com/xhanio/framingo/pkg/services/metrics"
	"github.com/xhanio/framingo/pkg/services/pubsub"
	"github.com/xhanio/framingo/pkg/services/supervisor"
	"github.com/xhanio/framingo/pkg/services/tracing"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/nameutil"

	"github.com/xhanio/framingo/example/pkg/services/example"
	"github.com/xhanio/framingo/example/pkg/services/repository"
	"github.com/xhanio/framingo/example/pkg/services/system/auth"
	"github.com/xhanio/framingo/example/pkg/services/system/certificate"
	"github.com/xhanio/framingo/example/pkg/services/system/organization"
	"github.com/xhanio/framingo/example/pkg/services/system/role"
	"github.com/xhanio/framingo/example/pkg/services/system/user"
	"github.com/xhanio/framingo/example/pkg/types/model"
)

type manager struct {
	name   string
	config *viper.Viper

	log log.Logger

	tracing    tracing.Manager
	db         db.Manager
	metrics    metrics.Manager
	pubsub     pubsub.Manager
	messagebus messagebus.Manager

	platform model.Platform

	repository repository.Repository

	user         user.Manager
	role         role.Manager
	organization organization.Manager
	certificate  certificate.Manager
	auth         auth.Manager

	example example.Manager

	api server.Manager

	services supervisor.Manager

	ctx    context.Context
	cancel context.CancelFunc
}

func New(configPath string) Server {
	m := &manager{
		config: newConfig(configPath),
	}
	m.name = nameutil.Name(m)
	return m
}

func (m *manager) Name() string {
	return m.name
}
