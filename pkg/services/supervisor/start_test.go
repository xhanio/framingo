package supervisor

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/types/common"
)

// A service whose Init failed is not started: its init error is the record,
// and the monitor's restart inits and starts it.
func TestStartSkipsAServiceWhoseInitFailed(t *testing.T) {
	m := newTestManager()
	bad := newMockService("bad")
	bad.initErr = errors.Newf("init boom")
	good := newMockService("good")
	require.NoError(t, m.Register(bad, good))
	require.NoError(t, m.TopoSort())
	require.Error(t, m.Init(context.Background()))

	require.NoError(t, m.Start(context.Background()), "the init error is not reported twice")
	assert.Equal(t, 0, bad.startCalled)
	assert.Equal(t, 1, good.startCalled)
	stat := m.c.stats.snapshot("bad")
	assert.False(t, stat.Stopped, "meant to run: the monitor restarts it")
	assert.NoError(t, stat.StartErr)
	require.NoError(t, m.Stop(context.Background()))
}

// A dependency whose start failed gates its dependents, through one with no
// Start of its own, as a failed init does; an independent service starts.
func TestStartGatesTheDependentsOfAFailedStart(t *testing.T) {
	m := newTestManager()
	db := newMockService("db")
	db.startErr = errors.Newf("start boom")
	repo := &initOnlyService{name: "repo", deps: []common.Service{db}}
	svc := newMockService("svc")
	svc.deps = []common.Service{repo}
	other := newMockService("other")
	require.NoError(t, m.Register(db, repo, svc, other))
	require.NoError(t, m.TopoSort())
	require.NoError(t, m.Init(context.Background()))

	err := m.Start(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start boom")
	assert.Contains(t, err.Error(), "dependency db is not running")
	assert.Contains(t, err.Error(), "dependency repo is not running")
	assert.Equal(t, 0, svc.startCalled)
	assert.Equal(t, 1, other.startCalled)
	stat := m.c.stats.snapshot("svc")
	assert.False(t, stat.Started)
	assert.False(t, stat.Stopped)
	assert.EqualError(t, stat.StartErr, "dependency repo is not running")
	require.NoError(t, m.Stop(context.Background()))
}

// A failed start leaves the supervisor started: Stop stops what did start.
func TestStopAfterAFailedStartStopsWhatStarted(t *testing.T) {
	m := newTestManager()
	ok := newMockService("ok")
	bad := newMockService("bad")
	bad.startErr = errors.Newf("start boom")
	require.NoError(t, m.Register(ok, bad))
	require.NoError(t, m.TopoSort())
	require.NoError(t, m.Init(context.Background()))
	require.Error(t, m.Start(context.Background()))

	require.NoError(t, m.Stop(context.Background()))
	assert.Equal(t, 1, ok.stopCalled)
}

// The monitor starts a gated service once its dependency runs.
func TestMonitorStartsAGatedServiceOnceItsDependencyRuns(t *testing.T) {
	var fixed atomic.Bool
	db := newMockService("db")
	db.startFn = func(context.Context) error {
		if !fixed.Load() {
			return errors.Newf("start boom")
		}
		return nil
	}
	svc := newMockService("svc")
	svc.deps = []common.Service{db}
	m := newTestManager(WithMonitorPolicy(10*time.Millisecond, -1, 0))
	require.NoError(t, m.Register(db, svc))
	require.NoError(t, m.TopoSort())
	require.NoError(t, m.Init(context.Background()))
	require.Error(t, m.Start(context.Background()))

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 0, m.c.stats.snapshot("svc").Restarts, "a restart its dependency gates is not counted")
	fixed.Store(true)
	require.Eventually(t, func() bool {
		stat := m.c.stats.snapshot("svc")
		return stat.Started && stat.StartErr == nil
	}, 2*time.Second, 10*time.Millisecond)
	require.NoError(t, m.Stop(context.Background()))
	assert.Equal(t, 1, svc.startCalled, "started once, after its dependency")
}

// A restart leaves the service alone while a dependency is not running.
func TestRestartWaitsForItsDependencies(t *testing.T) {
	m := newTestManager()
	db := newMockService("db")
	svc := newMockService("svc")
	svc.deps = []common.Service{db}
	require.NoError(t, m.Register(db, svc))
	require.NoError(t, m.TopoSort())
	require.NoError(t, m.Init(context.Background()))
	require.NoError(t, m.Start(context.Background()))
	require.NoError(t, m.StopService(context.Background(), "db"))

	err := m.RestartService(context.Background(), "svc")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.Unavailable))
	assert.Equal(t, 0, svc.stopCalled)
	assert.Equal(t, 1, svc.initCalled)
	assert.Equal(t, 0, m.c.stats.snapshot("svc").Restarts)
	require.NoError(t, m.Stop(context.Background()))
}

func TestStartServiceRefusesAServiceNotInitialized(t *testing.T) {
	m := newTestManager()
	svc := newMockService("svc")
	svc.initErr = errors.Newf("init boom")
	require.NoError(t, m.Register(svc))
	require.NoError(t, m.TopoSort())
	require.Error(t, m.Init(context.Background()))

	err := m.StartService(context.Background(), "svc")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.Conflict))
	assert.Equal(t, 0, svc.startCalled)
}

// The start policy bounds each Start; a stuck one fails, and gates its
// dependents, instead of holding the start up.
func TestStartPolicyBoundsAStuckStart(t *testing.T) {
	m := newTestManager(WithStartPolicy(50 * time.Millisecond))
	slow := newMockService("slow")
	slow.startFn = stuck
	svc := newMockService("svc")
	svc.deps = []common.Service{slow}
	require.NoError(t, m.Register(slow, svc))
	require.NoError(t, m.TopoSort())
	require.NoError(t, m.Init(context.Background()))

	began := time.Now()
	err := m.Start(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(began), time.Second)
	assert.Equal(t, 0, svc.startCalled)
	slow.startFn = nil
	require.NoError(t, m.Stop(context.Background()))
}

// The supervisor's Start passes its ctx to each service's Start.
func TestStartGivesEachServiceItsContext(t *testing.T) {
	type key struct{}
	m := newTestManager()
	svc := newMockService("svc")
	var got any
	svc.startFn = func(ctx context.Context) error {
		got = ctx.Value(key{})
		return nil
	}
	require.NoError(t, m.Register(svc))
	require.NoError(t, m.TopoSort())
	require.NoError(t, m.Init(context.Background()))
	require.NoError(t, m.Start(context.WithValue(context.Background(), key{}, "v")))
	assert.Equal(t, "v", got)
	require.NoError(t, m.Stop(context.Background()))
}

// A restart's Start takes the monitor's context, so Stop ends one stuck in it
// instead of waiting on it for good.
func TestStopEndsARestartStuckInStart(t *testing.T) {
	var starts atomic.Int32
	svc := newMockService("svc")
	svc.aliveErr = errors.Newf("dead")
	svc.startFn = func(ctx context.Context) error {
		if starts.Add(1) == 1 {
			return nil
		}
		return stuck(ctx)
	}
	m := newTestManager(WithMonitorPolicy(10*time.Millisecond, -1, 0))
	require.NoError(t, m.Register(svc))
	require.NoError(t, m.TopoSort())
	require.NoError(t, m.Init(context.Background()))
	require.NoError(t, m.Start(context.Background()))
	require.Eventually(t, func() bool { return starts.Load() >= 2 }, 2*time.Second, 5*time.Millisecond)

	stopped := make(chan error, 1)
	go func() { stopped <- m.Stop(context.Background()) }()
	select {
	case err := <-stopped:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Stop waited on a restart stuck in Start")
	}
}

// A whole restart whose Init fails still starts what can start.
func TestRestartStartsWhatItCanAfterAFailedInit(t *testing.T) {
	m := newTestManager()
	bad := newMockService("bad")
	good := newMockService("good")
	require.NoError(t, m.Register(bad, good))
	require.NoError(t, m.TopoSort())
	require.NoError(t, m.Init(context.Background()))
	require.NoError(t, m.Start(context.Background()))

	bad.initErr = errors.Newf("init boom")
	require.Error(t, m.Restart(context.Background()))
	assert.Equal(t, 2, good.startCalled)
	assert.Equal(t, 1, bad.startCalled)
	require.NoError(t, m.Stop(context.Background()))
}
