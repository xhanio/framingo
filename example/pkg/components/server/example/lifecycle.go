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

	if err := m.initServices(); err != nil {
		return errors.Wrap(err)
	}

	if err := m.services.Register(
		m.repository,
		m.user,
		m.role,
		m.organization,
		m.certificate,
		m.auth,
	); err != nil {
		return errors.Wrap(err)
	}

	if err := m.services.Register(
		m.example,
	); err != nil {
		return errors.Wrap(err)
	}

	// telemetry registers last: the topo sort pops dependency-less services in
	// reverse order, so it inits first and flushes last
	if err := m.services.Register(
		m.db,
		m.pubsub,
		m.messagebus,
		m.metrics,
		m.tracing,
	); err != nil {
		return errors.Wrap(err)
	}

	if err := m.services.TopoSort(); err != nil {
		return errors.Wrap(err)
	}

	// api after the sort starts last, as an ordering not a dependency: the
	// listener must come up even when a service behind a route failed init
	if err := m.services.Register(
		m.api,
	); err != nil {
		return errors.Wrap(err)
	}

	for _, svc := range m.services.Services() {
		m.messagebus.Register(svc)
	}

	if err := m.services.Init(ctx); err != nil {
		m.log.Error(err)
	}

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
	// until Stop: Start's ctx bounds the start alone
	m.ctx, m.cancel = context.WithCancel(context.WithoutCancel(ctx))
	m.listenSignals(m.ctx)
	return nil
}

func (m *manager) Stop(ctx context.Context) error {
	err := m.services.Stop(ctx)
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	return err
}

func (m *manager) Info(w io.Writer, debug bool) {
	m.services.Info(w, debug)
}
