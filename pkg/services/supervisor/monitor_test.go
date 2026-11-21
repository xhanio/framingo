package supervisor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
)

func TestSweepProbesSharedDependencyOnce(t *testing.T) {
	db := newMockService("db")
	repo := newMockService("repo")
	repo.deps = []common.Service{db}
	a := newMockService("a")
	a.deps = []common.Service{repo}
	b := newMockService("b")
	b.deps = []common.Service{repo}

	m := newTestManager()
	require.NoError(t, m.Register(db, repo, a, b))
	require.NoError(t, m.TopoSort())

	m.monitor.checkAll(context.Background())

	assert.Equal(t, 1, db.readyCalled, "db probed once, not once per transitive dependent")
	assert.Equal(t, 1, db.aliveCalled)
	assert.Equal(t, 1, repo.readyCalled, "repo probed once, not once per dependent")
	assert.Equal(t, 1, a.readyCalled)
	assert.Equal(t, 1, b.readyCalled)
}

func TestSweepRollsUpMemoizedDependencyFailure(t *testing.T) {
	db := newMockService("db")
	db.readyErr = errors.Newf("db down")
	a := newMockService("a")
	a.deps = []common.Service{db}
	b := newMockService("b")
	b.deps = []common.Service{db}

	m := newTestManager()
	require.NoError(t, m.Register(db, a, b))
	require.NoError(t, m.TopoSort())

	m.monitor.checkAll(context.Background())

	assert.Equal(t, 1, db.readyCalled)
	for _, name := range []string{"a", "b"} {
		stat := m.c.stats.snapshot(name)
		require.NotNil(t, stat)
		require.Error(t, stat.HealthcheckErr, "%s must inherit db's failure", name)
		assert.Contains(t, stat.HealthcheckErr.Error(), "db down")
	}
}

func TestHealthcheckSurvivesDependencyCycle(t *testing.T) {
	x := newMockService("x")
	y := newMockService("y")
	x.deps = []common.Service{y}
	y.deps = []common.Service{x}

	m := newTestManager()

	done := make(chan struct{})
	go func() {
		_ = m.monitor.healthcheck(context.Background(), x)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("healthcheck did not terminate on a dependency cycle")
	}
}

func TestMonitorRestartSkipsStoppedService(t *testing.T) {
	svc := newMockService("svc")
	m := newTestManager()
	require.NoError(t, m.Register(svc))
	require.NoError(t, m.TopoSort())
	require.NoError(t, m.Init(context.Background()))
	require.NoError(t, m.StartService(context.Background(), "svc"))
	require.NoError(t, m.StopService(context.Background(), "svc"))

	require.NoError(t, m.c.RestartIfRunning(context.Background(), svc))

	assert.Equal(t, 1, svc.stopCalled, "no second stop from a skipped restart")
	assert.Equal(t, 1, svc.initCalled)
	assert.Equal(t, 1, svc.startCalled)
	stat := m.c.stats.snapshot("svc")
	require.NotNil(t, stat)
	assert.True(t, stat.Stopped, "the deliberate stop survives")
}

func TestManualOpsSerializeWithRestart(t *testing.T) {
	svc := newMockService("svc")
	m := newTestManager()
	require.NoError(t, m.Register(svc))
	require.NoError(t, m.TopoSort())
	require.NoError(t, m.Init(context.Background()))
	require.NoError(t, m.StartService(context.Background(), "svc"))

	m.c.op.Lock()
	done := make(chan struct{})
	go func() {
		_ = m.StopService(context.Background(), "svc")
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, 0, svc.stopCalled, "StopService must wait for the operation lock")
	m.c.op.Unlock()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("StopService did not proceed after the lock was released")
	}
	assert.Equal(t, 1, svc.stopCalled)
}

func TestRestartDelayHonorsCancel(t *testing.T) {
	svc := newMockService("svc")
	svc.aliveErr = errors.Newf("dead")

	m := newTestManager(WithMonitorPolicy(0, -1, 2*time.Second))
	require.NoError(t, m.Register(svc))
	require.NoError(t, m.TopoSort())

	ctx, cancel := context.WithCancel(context.Background())
	start := time.Now()
	done := make(chan struct{})
	go func() {
		m.monitor.checkAll(ctx)
		close(done)
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("checkAll did not return after cancellation")
	}
	assert.Less(t, time.Since(start), time.Second, "cancellation must cut the restart delay short")
}

// Start's ctx bounds the start alone: canceling it leaves the monitor sweeping
// until Stop.
func TestMonitorOutlivesTheStartContext(t *testing.T) {
	m := newTestManager(WithMonitorPolicy(20*time.Millisecond, 0, 0))
	svc := newMockService("svc")
	require.NoError(t, m.Register(svc))
	require.NoError(t, m.TopoSort())
	require.NoError(t, m.Init(context.Background()))
	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, m.Start(ctx))
	cancel()
	time.Sleep(150 * time.Millisecond)
	require.NoError(t, m.Stop(context.Background()))
	assert.GreaterOrEqual(t, svc.aliveCalled, 2, "the monitor kept probing after Start's ctx ended")
}

// A service whose restarts are spent is passed over at once, not after the
// restart delay: the sweep goes on to the services after it.
func TestASpentServiceDoesNotHoldTheSweep(t *testing.T) {
	svc := newMockService("svc")
	svc.aliveErr = errors.Newf("dead")
	m := newTestManager(WithMonitorPolicy(0, 1, 2*time.Second))
	require.NoError(t, m.Register(svc))
	require.NoError(t, m.TopoSort())
	m.c.stats.update("svc", func(stat *entity.SupervisorStats) { stat.Restarts = 1 })

	began := time.Now()
	m.monitor.checkAll(context.Background())
	assert.Less(t, time.Since(began), time.Second)
	assert.Equal(t, 0, svc.initCalled, "not restarted")
}
