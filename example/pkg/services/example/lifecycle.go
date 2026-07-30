package example

import (
	"context"
	"io"

	"github.com/xhanio/framingo/pkg/utils/confutil"
)

func (m *manager) Init(ctx context.Context) error {
	// dynamic config change
	config := confutil.FromContext(ctx)
	m.apply(
		WithDynamicConfig(config.GetString("example.greeting")),
	)
	return nil
}

func (m *manager) Start(ctx context.Context) error {
	if m.cancel != nil {
		m.log.Warnf("%s already started", m.Name())
		return nil
	}
	ctx, cancel := context.WithCancel(m.ctx)
	m.cancel = cancel
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		// Deliberately for/select with a single case, not a bare <-ctx.Done():
		// this is the template's service run loop, and it is shaped so a
		// ticker, a subscription channel or a work queue can be added as
		// another case without restructuring anything.
		//
		//nolint:gosimple // S1000: kept as a loop for the cases that come later
		for {
			select {
			case <-ctx.Done():
				// put stop logic here
				m.log.Infof("service %s stopped", m.Name())
				return
			}
		}
	}()
	return nil
}

func (m *manager) Stop(wait bool) error {
	if m.cancel == nil {
		return nil
	}
	m.cancel()
	if wait {
		m.wg.Wait()
	}
	m.cancel = nil
	return nil
}

func (m *manager) Info(w io.Writer, debug bool) {
	// TODO: info
}
