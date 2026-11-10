package supervisor

import (
	"context"
	"io"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/printutil"
)

func (m *manager) Init(ctx context.Context) error {
	return m.c.initAll(ctx)
}

// The lifecycle methods - Init, Start, Stop, Restart - are not safe for
// concurrent use: m.cancel is unsynchronized. Call them from one goroutine,
// or serialize a signal handler's Stop externally.
func (m *manager) Start(ctx context.Context) error {
	if m.cancel != nil {
		m.log.Warnf("%s already started", m.Name())
		return nil
	}
	if err := m.c.startAll(); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		<-ctx.Done()
		m.log.Infof("service %s stopped", m.Name())
	}()
	if m.monitor.policy.Interval > 0 {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			m.monitor.run(ctx)
		}()
	}
	return nil
}

// Stop stops the services in reverse order, within the stop policy's timeout if one is set. A
// stopped supervisor returns at once: what its services left is theirs to wait for.
func (m *manager) Stop(ctx context.Context) error {
	if m.cancel == nil {
		m.log.Warnf("%s already stopped", m.Name())
		return nil
	}
	m.cancel()
	m.wg.Wait() // the monitor: a restart it is in holds the lock the stops need anyway
	m.cancel = nil
	return m.c.StopAll(ctx)
}

func (m *manager) Restart(ctx context.Context) error {
	m.log.Infof("restarting %s", m.Name())
	if err := m.Stop(ctx); err != nil {
		m.log.Errorf("failed to stop %s for restart: %s", m.Name(), err)
	}
	if err := m.Init(ctx); err != nil {
		return err
	}
	if err := m.Start(ctx); err != nil {
		return err
	}
	m.log.Infof("%s restarted", m.Name())
	return nil
}

func (m *manager) Info(w io.Writer, debug bool) {
	t := printutil.NewTable(w)
	t.Header("service status")
	t.Title("service", "alive", "ready", "uptime", "init_err", "start_err", "healthcheck_err")
	sw := newSweep() // one sweep for the whole table: shared deps probed once
	for _, service := range m.c.services {
		_ = m.monitor.check(context.Background(), service, sw) // refreshes stat fields for display
	}
	stats, _ := m.Stats() // errors are displayed in the table below
	for _, stat := range stats {
		alive := stat.LivenessErr == nil && stat.Healthcheck() == nil
		t.Row(stat.Name, alive, stat.Ready, stat.Uptime(), stat.InitializationErr, stat.StartErr, stat.HealthcheckErr)
	}
	t.NewLine()
	t.Flush()
	for _, service := range m.c.services {
		if svc, ok := service.(common.Debuggable); ok {
			svc.Info(w, debug)
		}
	}
}
