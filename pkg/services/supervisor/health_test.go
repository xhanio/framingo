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

func TestSupervisorReady(t *testing.T) {
	alpha := newMockService("alpha")
	beta := newMockService("beta")
	m := newTestManager()
	require.NoError(t, m.Register(alpha, beta))
	require.NoError(t, m.TopoSort())

	require.Error(t, m.Ready(context.Background()))

	m.c.stats.update("alpha", func(stat *entity.SupervisorStats) { stat.Ready = true })
	m.c.stats.update("beta", func(stat *entity.SupervisorStats) { stat.Ready = true })
	require.NoError(t, m.Ready(context.Background()))

	m.c.stats.update("beta", func(stat *entity.SupervisorStats) {
		stat.Ready = false
		stat.ReadinessErr = errors.Newf("database ping failed")
	})
	err := m.Ready(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "beta")
	assert.Contains(t, err.Error(), "ping")
	assert.NotContains(t, err.Error(), "alpha")
}

func TestSupervisorAlive(t *testing.T) {
	t.Run("bounded retries escalate when exhausted", func(t *testing.T) {
		svc := newMockService("omega")
		m := newTestManager(WithMonitorPolicy(0, 3, 0))
		require.NoError(t, m.Register(svc))
		require.NoError(t, m.TopoSort())
		require.NoError(t, m.Alive(context.Background()))

		m.c.stats.update("omega", func(stat *entity.SupervisorStats) {
			stat.LivenessErr = errors.Newf("dead")
			stat.Restarts = 2
		})
		assert.NoError(t, m.Alive(context.Background()), "recovery still in progress must not escalate")

		m.c.stats.update("omega", func(stat *entity.SupervisorStats) { stat.Restarts = 3 })
		err := m.Alive(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "omega")
	})

	t.Run("unlimited retries never escalate", func(t *testing.T) {
		svc := newMockService("omega")
		m := newTestManager(WithMonitorPolicy(0, -1, 0))
		require.NoError(t, m.Register(svc))
		require.NoError(t, m.TopoSort())
		m.c.stats.update("omega", func(stat *entity.SupervisorStats) {
			stat.LivenessErr = errors.Newf("dead")
			stat.Restarts = 100
		})
		assert.NoError(t, m.Alive(context.Background()))
	})

	t.Run("restarts disabled escalate immediately", func(t *testing.T) {
		svc := newMockService("omega")
		m := newTestManager() // no restart policy: maxRetries 0
		require.NoError(t, m.Register(svc))
		require.NoError(t, m.TopoSort())
		m.c.stats.update("omega", func(stat *entity.SupervisorStats) { stat.LivenessErr = errors.Newf("dead") })
		require.Error(t, m.Alive(context.Background()),
			"with no in-process recovery configured, the platform is the recovery path")
	})
}

// flappingService flips both probes to hit every stat write path; atomic
// counters leave only the supervisor's state for the race detector.
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

// run under -race: health endpoints and the monitor share per-service stats
func TestHealthServesConcurrentlyWithMonitor(t *testing.T) {
	svc := &flappingService{name: "flappy"}
	m := newTestManager(
		WithMonitorPolicy(time.Millisecond, -1, 0),
	)
	require.NoError(t, m.Register(svc))
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
	require.NoError(t, m.Stop(context.Background()))
}

func TestInfoSharesOneSweep(t *testing.T) {
	dep := newMockService("shared-dep")
	a := newMockService("svc-a")
	a.deps = []common.Service{dep}
	b := newMockService("svc-b")
	b.deps = []common.Service{dep}
	m := newTestManager()
	require.NoError(t, m.Register(dep, a, b))
	require.NoError(t, m.TopoSort())

	m.Info(io.Discard, false)

	assert.Equal(t, 1, dep.readyCalled, "shared dependency probed once for the whole table")
}
