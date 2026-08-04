package supervisor

import (
	"context"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
)

// Ready is the graph's roll-up: nil iff every registered service is ready,
// judged from the stats the monitor maintains — no probing of its own.
func TestSupervisorReady(t *testing.T) {
	alpha := newMockService("alpha")
	beta := newMockService("beta")
	m := newTestManager()
	m.Register(alpha, beta)
	require.NoError(t, m.TopoSort())

	// Nothing has started: nothing is ready.
	require.Error(t, m.Ready(context.Background()))

	m.c.update("alpha", func(stat *entity.SupervisorStats) { stat.Ready = true })
	m.c.update("beta", func(stat *entity.SupervisorStats) { stat.Ready = true })
	require.NoError(t, m.Ready(context.Background()))

	m.c.update("beta", func(stat *entity.SupervisorStats) {
		stat.Ready = false
		stat.ReadinessErr = errors.Newf("database ping failed")
	})
	err := m.Ready(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "beta")
	assert.Contains(t, err.Error(), "ping")
	assert.NotContains(t, err.Error(), "alpha")
}

// Alive escalates only when in-process recovery is spent: a liveness
// failure with restarts exhausted (or never configured). While the monitor
// is still restarting, and under unlimited retries, the process stays alive.
func TestSupervisorAlive(t *testing.T) {
	t.Run("bounded retries escalate when exhausted", func(t *testing.T) {
		svc := newMockService("omega")
		m := newTestManager(WithMonitorPolicy(0, 3, 0))
		m.Register(svc)
		require.NoError(t, m.TopoSort())
		require.NoError(t, m.Alive(context.Background()))

		m.c.update("omega", func(stat *entity.SupervisorStats) {
			stat.LivenessErr = errors.Newf("dead")
			stat.Restarts = 2
		})
		assert.NoError(t, m.Alive(context.Background()), "recovery still in progress must not escalate")

		m.c.update("omega", func(stat *entity.SupervisorStats) { stat.Restarts = 3 })
		err := m.Alive(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "omega")
	})

	t.Run("unlimited retries never escalate", func(t *testing.T) {
		svc := newMockService("omega")
		m := newTestManager(WithMonitorPolicy(0, -1, 0))
		m.Register(svc)
		require.NoError(t, m.TopoSort())
		m.c.update("omega", func(stat *entity.SupervisorStats) {
			stat.LivenessErr = errors.Newf("dead")
			stat.Restarts = 100
		})
		assert.NoError(t, m.Alive(context.Background()))
	})

	t.Run("restarts disabled escalate immediately", func(t *testing.T) {
		svc := newMockService("omega")
		m := newTestManager() // no restart policy: maxRetries 0
		m.Register(svc)
		require.NoError(t, m.TopoSort())
		m.c.update("omega", func(stat *entity.SupervisorStats) { stat.LivenessErr = errors.Newf("dead") })
		require.Error(t, m.Alive(context.Background()),
			"with no in-process recovery configured, the platform is the recovery path")
	})
}

// flappingService alternates both probes between healthy and failing so a
// monitor sweep exercises every stat write path — liveness errors, readiness
// flips, and the restart bookkeeping behind them. Its counters are atomic so
// the only unsynchronized state left is the supervisor's own.
type flappingService struct {
	name  string
	alive atomic.Int64
	ready atomic.Int64
}

func (s *flappingService) Name() string                   { return s.name }
func (s *flappingService) Dependencies() []common.Service { return nil }

func (s *flappingService) Alive(ctx context.Context) error {
	if s.alive.Add(1)%2 == 0 {
		return errors.Newf("flapping liveness")
	}
	return nil
}

func (s *flappingService) Ready(ctx context.Context) error {
	if s.ready.Add(1)%2 == 0 {
		return errors.Newf("flapping readiness")
	}
	return nil
}

// A supervisor serves Alive/Ready/Stats/Info from request goroutines
// (/healthz, /readyz, debug endpoints) while its monitor goroutine sweeps
// probes and restarts failing services. Every one of those paths touches the
// same per-service stats, so run the whole ensemble concurrently and let the
// race detector judge the synchronization.
func TestHealthServesConcurrentlyWithMonitor(t *testing.T) {
	svc := &flappingService{name: "flappy"}
	m := newTestManager(
		WithMonitorPolicy(time.Millisecond, -1, 0),
	)
	m.Register(svc)
	require.NoError(t, m.TopoSort())
	require.NoError(t, m.Init(context.Background()))
	require.NoError(t, m.Start(context.Background()))

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	serve := func(f func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				f()
			}
		}()
	}
	serve(func() { _ = m.Alive(ctx) })
	serve(func() { _ = m.Ready(ctx) })
	serve(func() {
		stats, _ := m.Stats()
		for _, stat := range stats {
			_ = stat.Healthcheck()
		}
	})
	serve(func() { m.Info(io.Discard, false) })

	time.Sleep(200 * time.Millisecond)
	cancel()
	wg.Wait()
	require.NoError(t, m.Stop(true))
}

// Info renders one row per service; the whole table must share a single
// sweep instead of re-probing shared dependencies once per row.
func TestInfoSharesOneSweep(t *testing.T) {
	dep := newMockService("shared-dep")
	a := newMockService("svc-a")
	a.deps = []common.Service{dep}
	b := newMockService("svc-b")
	b.deps = []common.Service{dep}
	m := newTestManager()
	m.Register(dep, a, b)
	require.NoError(t, m.TopoSort())

	m.Info(io.Discard, false)

	assert.Equal(t, 1, dep.readyCalled, "shared dependency probed once for the whole table")
}
