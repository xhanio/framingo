package supervisor

import (
	"context"
	"io"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/printutil"
)

func (m *manager) Init(ctx context.Context) error {
	return m.c.InitAll(ctx)
}

// Init/Start/Stop/Restart are not concurrency-safe: the caller serializes them.
func (m *manager) Start(ctx context.Context) error {
	if m.loop.Running() {
		m.log.Warnf("%s already started", m.Name())
		return nil
	}
	err := m.c.StartAll(ctx)
	// whatever failed: Stop then stops what did start, and the monitor restarts the rest
	m.loop.Start(ctx, m.watch)
	return err
}

// watch runs the health monitor, when the policy sets one, until Stop.
func (m *manager) watch(ctx context.Context) {
	if m.monitor.policy.Interval > 0 {
		m.monitor.run(ctx)
	} else {
		<-ctx.Done()
	}
	m.log.Infof("service %s stopped", m.Name())
}

// Stop on a stopped supervisor returns at once.
func (m *manager) Stop(ctx context.Context) error {
	// waits for the monitor: a restart it is in holds the lock the stops need anyway
	if !m.loop.Stop() {
		m.log.Warnf("%s already stopped", m.Name())
		return nil
	}
	return m.c.StopAll(ctx)
}

func (m *manager) Restart(ctx context.Context) error {
	m.log.Infof("restarting %s", m.Name())
	if err := m.Stop(ctx); err != nil {
		m.log.Errorf("failed to stop %s for restart: %s", m.Name(), err)
	}
	// a failed Init still starts what it can, as at the first start
	initErr := m.Init(ctx)
	if err := errors.Combine(initErr, m.Start(ctx)); err != nil {
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
