package supervisor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/types/common"
)

// A monitoring sweep must probe every service exactly once: shared
// dependencies are not re-probed for each dependent, but their results still
// roll up into every dependent's healthcheck error.
func TestSweepProbesSharedDependencyOnce(t *testing.T) {
	db := newMockService("db")
	repo := newMockService("repo")
	repo.deps = []common.Service{db}
	a := newMockService("a")
	a.deps = []common.Service{repo}
	b := newMockService("b")
	b.deps = []common.Service{repo}

	m := newTestManager()
	m.Register(db, repo, a, b)
	require.NoError(t, m.TopoSort())

	m.monitor.checkAll(context.Background())

	assert.Equal(t, 1, db.readyCalled, "db probed once, not once per transitive dependent")
	assert.Equal(t, 1, db.aliveCalled)
	assert.Equal(t, 1, repo.readyCalled, "repo probed once, not once per dependent")
	assert.Equal(t, 1, a.readyCalled)
	assert.Equal(t, 1, b.readyCalled)
}

// Memoization must not swallow the rollup: a dependency's failure still
// surfaces in each dependent's healthcheck error.
func TestSweepRollsUpMemoizedDependencyFailure(t *testing.T) {
	db := newMockService("db")
	db.readyErr = errors.Newf("db down")
	a := newMockService("a")
	a.deps = []common.Service{db}
	b := newMockService("b")
	b.deps = []common.Service{db}

	m := newTestManager()
	m.Register(db, a, b)
	require.NoError(t, m.TopoSort())

	m.monitor.checkAll(context.Background())

	assert.Equal(t, 1, db.readyCalled)
	for _, name := range []string{"a", "b"} {
		stat := m.c.stat(name)
		require.NotNil(t, stat)
		require.Error(t, stat.HealthcheckErr, "%s must inherit db's failure", name)
		assert.Contains(t, stat.HealthcheckErr.Error(), "db down")
	}
}

// TopoSort rejects cycles at wiring time; if one slips through anyway, an
// ad-hoc healthcheck must terminate instead of recursing forever.
func TestHealthcheckSurvivesDependencyCycle(t *testing.T) {
	x := newMockService("x")
	y := newMockService("y")
	x.deps = []common.Service{y}
	y.deps = []common.Service{x}

	m := newTestManager()

	done := make(chan struct{})
	go func() {
		_ = m.monitor.healthcheck(x)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("healthcheck did not terminate on a dependency cycle")
	}
}

// A restart delay must not outlive the monitor's context: cancellation during
// the delay returns promptly instead of sleeping through it.
func TestRestartDelayHonorsCancel(t *testing.T) {
	svc := newMockService("svc")
	svc.aliveErr = errors.Newf("dead")

	m := newTestManager(WithRestartPolicy(-1), WithRestartDelay(2*time.Second))
	m.Register(svc)
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
