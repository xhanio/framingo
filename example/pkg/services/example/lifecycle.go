package example

import (
	"context"
	"io"

	"github.com/xhanio/framingo/pkg/utils/confutil"
)

func (m *manager) Init(ctx context.Context) error {
	s := m.resolve(confutil.FromContext(ctx))
	// published whole: requests read the settings while a restart reloads them
	m.mu.Lock()
	m.settings = s
	m.mu.Unlock()
	if m.inst == nil {
		inst, err := m.newInstruments()
		if err != nil {
			return err
		}
		m.inst = inst
	}
	return nil
}

func (m *manager) Start(ctx context.Context) error {
	if !m.loop.Start(ctx, m.run) {
		m.log.Warnf("%s already started", m.Name())
	}
	return nil
}

func (m *manager) Stop(ctx context.Context) error {
	m.loop.Stop()
	return m.wg.Wait(ctx)
}

// run is the template's run loop: add tickers or queues as cases, and hand
// work that takes time to m.wg.Go, with ctx as an argument.
func (m *manager) run(ctx context.Context) {
	//nolint:staticcheck // S1000: kept as a loop for the cases that come later
	for {
		select {
		case <-ctx.Done():
			// return at once: Stop waits for the loop with no deadline, so slow cleanup goes in m.wg.Go
			m.log.Infof("service %s stopped", m.Name())
			return
		}
	}
}

func (m *manager) Info(w io.Writer, debug bool) {
	// TODO: info
}
