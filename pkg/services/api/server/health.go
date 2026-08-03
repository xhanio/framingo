package server

import (
	"github.com/xhanio/errors"
)

// Alive and Ready both report on the listeners. A listener that stopped
// serving fails liveness because a restart genuinely fixes it - Init
// rebuilds the echo instances, so the supervisor's restart re-binds it -
// and a server not accepting connections is equally not ready for traffic.
func (m *manager) Alive() error {
	return m.listenersServing()
}

func (m *manager) Ready() error {
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
