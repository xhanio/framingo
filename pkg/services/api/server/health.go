package server

import (
	"context"

	"github.com/xhanio/errors"
)

// a dead listener fails liveness too: a restart's Init rebuilds echo and re-binds it
func (m *manager) Alive(_ context.Context) error {
	return m.listenersServing()
}

func (m *manager) Ready(_ context.Context) error {
	return m.listenersServing()
}

func (m *manager) listenersServing() error {
	var errs []error
	for _, s := range m.servers {
		if err := s.serveError(); err != nil {
			errs = append(errs, errors.Wrapf(err, "server %s not serving", s.name))
		}
	}
	return errors.Combine(errs...)
}
