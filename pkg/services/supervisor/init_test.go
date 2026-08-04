package supervisor

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xhanio/framingo/pkg/types/common"
)

// failNTimes returns an error for the first n calls, then nil - a database
// bootstrapping, a socket not yet listening.
func failNTimes(n int) func() error {
	calls := 0
	return func() error {
		calls++
		if calls <= n {
			return fmt.Errorf("not yet (call %d)", calls)
		}
		return nil
	}
}

// With the init policy on, a dependent's turn waits for its dependency to
// answer the Ready probe before running its own Init - a bootstrapping
// database holds the repository's init until the ping answers.
func TestInitWaitsForDependencyReadiness(t *testing.T) {
	db := newMockService("db")
	db.readyFn = failNTimes(2)
	repo := newMockService("repo")
	repo.deps = []common.Service{db}

	m := newTestManager(
		WithInitPolicy(-1, time.Millisecond, 4*time.Millisecond),
	)
	m.Register(db, repo)
	require.NoError(t, m.TopoSort())

	require.NoError(t, m.Init(context.Background()))
	assert.GreaterOrEqual(t, db.readyCalled, 3, "repo's turn re-probes until the db answers")
	assert.Equal(t, 1, repo.initCalled)
}

// A service's own failing Init retries at its own turn - so by the time
// dependents run, the dependency is initialized and only Ready is observed.
func TestInitRetriesOwnInit(t *testing.T) {
	db := newMockService("db")
	db.initFn = failNTimes(2)
	repo := newMockService("repo")
	repo.deps = []common.Service{db}

	m := newTestManager(
		WithInitPolicy(2, time.Millisecond, time.Millisecond),
	)
	m.Register(db, repo)
	require.NoError(t, m.TopoSort())

	require.NoError(t, m.Init(context.Background()))
	assert.Equal(t, 3, db.initCalled, "first attempt plus two retries")
	assert.Equal(t, 1, repo.initCalled)
}

// A bounded policy gives up after its retries; the failing service records
// the error and dependents fail fast without burning their own budget.
func TestInitPolicyExhausted(t *testing.T) {
	db := newMockService("db")
	db.initErr = fmt.Errorf("db down")
	repo := newMockService("repo")
	repo.deps = []common.Service{db}

	m := newTestManager(
		WithInitPolicy(2, time.Millisecond, time.Millisecond),
	)
	m.Register(db, repo)
	require.NoError(t, m.TopoSort())

	start := time.Now()
	err := m.Init(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db")
	assert.Equal(t, 3, db.initCalled, "first attempt plus two retries")
	assert.Equal(t, 0, repo.initCalled, "dependent fails fast on a dependency that gave up")
	assert.Less(t, time.Since(start), time.Second, "repo's turn must not re-wait a lost cause")

	stat := m.c.snapshot("repo")
	require.NotNil(t, stat)
	assert.Error(t, stat.InitializationErr)
}

// Policy off is today's one-pass behavior: the Initialized flag is the only
// dependency gate, and no Ready probes run during init.
func TestInitPolicyDisabledSkipsProbes(t *testing.T) {
	db := newMockService("db")
	db.readyErr = fmt.Errorf("never ready")
	repo := newMockService("repo")
	repo.deps = []common.Service{db}

	m := newTestManager()
	m.Register(db, repo)
	require.NoError(t, m.TopoSort())

	require.NoError(t, m.Init(context.Background()))
	assert.Equal(t, 0, db.readyCalled, "no probing during a one-pass init")
	assert.Equal(t, 1, repo.initCalled)
}

// Cancelling the Init context cuts an unbounded wait short.
func TestInitWaitHonorsCancel(t *testing.T) {
	db := newMockService("db")
	db.readyErr = fmt.Errorf("never ready")
	repo := newMockService("repo")
	repo.deps = []common.Service{db}

	m := newTestManager(
		WithInitPolicy(-1, 50*time.Millisecond, 50*time.Millisecond),
	)
	m.Register(db, repo)
	require.NoError(t, m.TopoSort())

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	err := m.Init(ctx)
	require.Error(t, err)
	assert.Less(t, time.Since(start), time.Second, "cancellation must end the wait promptly")
}
