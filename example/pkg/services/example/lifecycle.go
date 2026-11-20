package example

import (
	"context"
	"io"

	"github.com/xhanio/framingo/pkg/utils/confutil"
	"github.com/xhanio/framingo/pkg/utils/nameutil"
)

func (m *manager) Init(ctx context.Context) error {
	config := confutil.FromContext(ctx)
	root := nameutil.ConfigRoot(m.name)
	// locked: requests read the greeting while a restart reloads it
	m.mu.Lock()
	m.applyOptions(
		WithDynamicConfig(config.GetString("%s.greeting", root)),
	)
	m.setDefaults()
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
	if m.cancel != nil {
		m.log.Warnf("%s already started", m.Name())
		return nil
	}
	ctx, cancel := context.WithCancel(m.ctx)
	m.cancel = cancel
	looped := make(chan struct{})
	m.looped = looped
	go func() {
		defer close(looped)
		// template run loop: add tickers or queues as cases; hand-off work runs in m.wg.Go
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

func (m *manager) Stop(ctx context.Context) error {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
		<-m.looped
	}
	return m.wg.Wait(ctx)
}

func (m *manager) Info(w io.Writer, debug bool) {
	// TODO: info
}
