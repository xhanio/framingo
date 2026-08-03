package supervisor

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/types/common"
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
	require.Error(t, m.Ready())

	m.c.stat("alpha").Ready = true
	m.c.stat("beta").Ready = true
	require.NoError(t, m.Ready())

	m.c.stat("beta").Ready = false
	m.c.stat("beta").ReadinessErr = errors.Newf("database ping failed")
	err := m.Ready()
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
		m := newTestManager(WithRestartPolicy(3))
		m.Register(svc)
		require.NoError(t, m.TopoSort())
		require.NoError(t, m.Alive())

		stat := m.c.stat("omega")
		stat.LivenessErr = errors.Newf("dead")
		stat.Restarts = 2
		assert.NoError(t, m.Alive(), "recovery still in progress must not escalate")

		stat.Restarts = 3
		err := m.Alive()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "omega")
	})

	t.Run("unlimited retries never escalate", func(t *testing.T) {
		svc := newMockService("omega")
		m := newTestManager(WithRestartPolicy(-1))
		m.Register(svc)
		require.NoError(t, m.TopoSort())
		stat := m.c.stat("omega")
		stat.LivenessErr = errors.Newf("dead")
		stat.Restarts = 100
		assert.NoError(t, m.Alive())
	})

	t.Run("restarts disabled escalate immediately", func(t *testing.T) {
		svc := newMockService("omega")
		m := newTestManager() // no restart policy: maxRetries 0
		m.Register(svc)
		require.NoError(t, m.TopoSort())
		m.c.stat("omega").LivenessErr = errors.Newf("dead")
		require.Error(t, m.Alive(),
			"with no in-process recovery configured, the platform is the recovery path")
	})
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
