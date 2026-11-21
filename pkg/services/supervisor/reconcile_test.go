package supervisor

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/types/common"
)

// calls is what Reconcile heard, across dependents, in order.
type calls struct {
	mu  sync.Mutex
	got []string
}

func (c *calls) add(s string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.got = append(c.got, s)
}

func (c *calls) all() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.got...)
}

type reconciling struct {
	*mockService
	calls *calls
	err   error
}

var _ common.Reconcilable = (*reconciling)(nil)

func (s *reconciling) Reconcile(_ context.Context, dep common.Service, state common.DependencyState) error {
	s.calls.add(fmt.Sprintf("%s: %s %s", s.name, dep.Name(), state))
	return s.err
}

func newReconciling(name string, c *calls, deps ...common.Service) *reconciling {
	s := &reconciling{mockService: newMockService(name), calls: c}
	s.deps = deps
	return s
}

func startAll(t *testing.T, m *manager, services ...common.Service) {
	t.Helper()
	require.NoError(t, m.Register(services...))
	require.NoError(t, m.TopoSort())
	require.NoError(t, m.Init(context.Background()))
	require.NoError(t, m.Start(context.Background()))
	t.Cleanup(func() { _ = m.Stop(context.Background()) })
}

// A restart tells the dependents that run on, direct ones alone, in
// dependency order: stopped, then running.
func TestReconcileTellsTheDependentsOfARestart(t *testing.T) {
	c := &calls{}
	db := newMockService("db")
	a := newReconciling("a", c, db)
	b := newReconciling("b", c, db, a)
	top := newReconciling("top", c, b)
	other := newReconciling("other", c)
	m := newTestManager()
	startAll(t, m, db, a, b, top, other)

	require.NoError(t, m.RestartService(context.Background(), "db"))
	assert.Equal(t, []string{
		"a: db stopped", "b: db stopped",
		"a: db running", "b: db running",
	}, c.all())
}

func TestReconcileTellsAFailedRestart(t *testing.T) {
	c := &calls{}
	db := newMockService("db")
	svc := newReconciling("svc", c, db)
	m := newTestManager()
	startAll(t, m, db, svc)

	db.initErr = errors.Newf("init boom")
	require.Error(t, m.RestartService(context.Background(), "db"))
	assert.Equal(t, []string{"svc: db stopped", "svc: db failed"}, c.all())
}

func TestReconcileTellsAStopAndAStartByHand(t *testing.T) {
	c := &calls{}
	db := newMockService("db")
	svc := newReconciling("svc", c, db)
	m := newTestManager()
	startAll(t, m, db, svc)

	require.NoError(t, m.StopService(context.Background(), "db"))
	require.NoError(t, m.StartService(context.Background(), "db"))
	db.startErr = errors.Newf("start boom")
	require.Error(t, m.StartService(context.Background(), "db"))
	assert.Equal(t, []string{"svc: db stopped", "svc: db running", "svc: db failed"}, c.all())
}

// A dependent that does not run is not told: it registers everything when it
// starts again.
func TestReconcileSkipsADependentNotRunning(t *testing.T) {
	c := &calls{}
	db := newMockService("db")
	svc := newReconciling("svc", c, db)
	m := newTestManager()
	startAll(t, m, db, svc)

	require.NoError(t, m.StopService(context.Background(), "svc"))
	require.NoError(t, m.RestartService(context.Background(), "db"))
	assert.Empty(t, c.all())
}

// A restart its own dependencies gate changes nothing, so tells no one.
func TestAGatedRestartReconcilesNothing(t *testing.T) {
	c := &calls{}
	base := newMockService("base")
	db := newMockService("db")
	db.deps = []common.Service{base}
	svc := newReconciling("svc", c, db)
	m := newTestManager()
	startAll(t, m, base, db, svc)

	require.NoError(t, m.c.Stop(canceled(), base))
	require.Error(t, m.RestartService(context.Background(), "db"))
	assert.Empty(t, c.all())
}

// A failed Reconcile fails the dependent: it is not running, its health says
// why, and its own restart clears it.
func TestAFailedReconcileFailsTheDependent(t *testing.T) {
	c := &calls{}
	db := newMockService("db")
	svc := newReconciling("svc", c, db)
	svc.err = errors.Newf("lost it")
	top := newMockService("top")
	top.deps = []common.Service{svc}
	m := newTestManager()
	startAll(t, m, db, svc, top)

	require.NoError(t, m.RestartService(context.Background(), "db"), "the dependent's failure is its own")
	stat := m.c.stats.snapshot("svc")
	require.Error(t, stat.ReconcileErr)
	assert.Contains(t, stat.ReconcileErr.Error(), "failed to reconcile with db stopped")
	assert.Contains(t, stat.ReconcileErr.Error(), "lost it")
	assert.False(t, stat.Ready)
	assert.Error(t, stat.Healthcheck())
	assert.False(t, m.c.running(svc))
	assert.Equal(t, []string{"svc: db stopped"}, c.all(), "a failed dependent hears no more")

	svc.err = nil
	require.NoError(t, m.RestartService(context.Background(), "svc"))
	stat = m.c.stats.snapshot("svc")
	assert.NoError(t, stat.ReconcileErr)
	assert.NoError(t, stat.Healthcheck())
}

// The whole graph's start, stop and restart tell no one: the dependents stop
// and start with it, and register as they start.
func TestTheWholeGraphReconcilesNothing(t *testing.T) {
	c := &calls{}
	db := newMockService("db")
	svc := newReconciling("svc", c, db)
	m := newTestManager()
	startAll(t, m, db, svc)

	require.NoError(t, m.Restart(context.Background()))
	require.NoError(t, m.Stop(context.Background()))
	assert.Empty(t, c.all())
}
