package example

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/xhanio/errors"
)

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
		m.bus,
		m.repository,
		m.user,
		m.role,
		m.organization,
		m.certificate,
		m.auth,
	)

	// register business services
	m.services.Register(
		m.example,
	)

	// perform a topo sort to ensure the dependencies
	if err := m.services.TopoSort(); err != nil {
		return errors.Wrap(err)
	}

	// append api & grpc after topo sort to ensure the latest start
	m.services.Register(
		m.api,
	)

	// subscribe all services to the service bus
	for _, svc := range m.services.Services() {
		m.bus.Subscribe(svc, "/")
		m.bus.Subscribe(svc, fmt.Sprintf("/components/%s", m.Name()))
		m.bus.Subscribe(svc, fmt.Sprintf("/components/%s/services/%s", m.Name(), svc.Name()))
	}

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
	if m.cancel != nil {
		m.log.Warn("manager already started, skipping")
		return nil
	}
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
	m.ctx, m.cancel = context.WithCancel(ctx)
	m.listenSignals(m.ctx)
	return nil
}

func (m *manager) Stop(wait bool) error {
	if err := m.services.Stop(wait); err != nil {
		m.log.Error(err)
	}
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	return nil
}

func (m *manager) Info(w io.Writer, debug bool) {
	m.services.Info(w, debug)
}
