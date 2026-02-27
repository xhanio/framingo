package example

import (
	"context"
	"fmt"
	"io"
	"net/http"
	_ "net/http/pprof"

	"github.com/spf13/viper"
	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/services/api/server"
	"github.com/xhanio/framingo/pkg/services/app"
	"github.com/xhanio/framingo/pkg/services/db"
	"github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/utils/log"

	"github.com/xhanio/framingo/example/pkg/services/example"
)

type manager struct {
	config *viper.Viper
	// util services
	log log.Logger
	db  db.Manager

	// system services

	// business services
	example example.Manager

	// api related services
	mws []api.Middleware
	api server.Manager

	// service controller
	services app.Manager
	cancel   context.CancelFunc
}

func New(configPath string) Server {
	return &manager{
		config: newConfig(configPath),
	}
}

func (m *manager) Init(ctx context.Context) error {
	if err := m.initConfig(); err != nil {
		return errors.Wrap(err)
	}

	// create all service instances
	if err := m.initServices(); err != nil {
		return errors.Wrap(err)
	}

	// register basic services
	m.services.Register(
		m.db,
	)

	// register system services
	m.services.Register(
		m.example,
	)

	// register business services

	// perform a topo sort to ensure the dependencies
	if err := m.services.TopoSort(); err != nil {
		return errors.Wrap(err)
	}

	// append api & grpc after topo sort to ensure the latest start
	m.services.Register(
		m.api,
	)

	// register to eventbus
	// if err := m.registerEventHandler(m.services.Services()...); err != nil {
	// 	return errors.Wrap(err)
	// }

	/* pre initialization */

	// init all services
	if err := m.services.Init(ctx); err != nil {
		m.log.Error(err)
	}

	/* post initialization */

	if err := m.initAPI(); err != nil {
		return errors.Wrap(err)
	}

	return nil
}

func (m *manager) Start(ctx context.Context) error {
	// enable pprof
	pport := m.config.GetUint("pprof.port")
	if pport != 0 {
		go func() {
			m.log.Infof("enable pprof on port %d", pport)
			err := http.ListenAndServe(fmt.Sprintf("localhost:%d", pport), nil)
			if err != nil {
				panic(err)
			}
		}()
	}
	if err := m.services.Start(ctx); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.listenSignals(ctx)
	<-ctx.Done()
	m.log.Info("gracefully shutdown manager")
	return nil
}

func (m *manager) Stop(wait bool) error {
	if err := m.services.Stop(wait); err != nil {
		m.log.Error(err)
	}
	if m.cancel != nil {
		m.cancel()
	}
	return nil
}

func (m *manager) Info(w io.Writer, debug bool) {
	m.services.Info(w, debug)
}
