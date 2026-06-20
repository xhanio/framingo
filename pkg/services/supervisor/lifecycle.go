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
	if m.monitor.interval > 0 {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			m.monitor.run(ctx)
		}()
	}
	return nil
}

func (m *manager) Stop(wait bool) error {
	if m.cancel == nil {
		m.log.Warnf("%s already stopped", m.Name())
		return nil
	}
	m.cancel()
	if wait {
		m.wg.Wait()
	}
	m.cancel = nil
	return m.c.stopAll(wait)
}

func (m *manager) Restart(ctx context.Context) error {
	m.log.Infof("restarting %s", m.Name())
	if err := m.Stop(true); err != nil {
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
	stats, _ := m.Stats() // errors are displayed in the table below
	for _, stat := range stats {
		_ = m.monitor.healthcheck(stat.Source) // refreshes stat fields for display
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
