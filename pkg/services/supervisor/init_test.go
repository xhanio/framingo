package supervisor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"
)

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

func TestInitWaitsForDependencyReadiness(t *testing.T) {
	db := newMockService("db")
	db.readyFn = failNTimes(2)
	repo := newMockService("repo")
	repo.deps = []common.Service{db}

	m := newTestManager(
		WithInitPolicy(-1, time.Millisecond, 4*time.Millisecond),
	)
	require.NoError(t, m.Register(db, repo))
	require.NoError(t, m.TopoSort())

	require.NoError(t, m.Init(context.Background()))
	assert.GreaterOrEqual(t, db.readyCalled, 3, "repo's turn re-probes until the db answers")
	assert.Equal(t, 1, repo.initCalled)
}

func TestInitRetriesOwnInit(t *testing.T) {
	db := newMockService("db")
	db.initFn = failNTimes(2)
	repo := newMockService("repo")
	repo.deps = []common.Service{db}

	m := newTestManager(
		WithInitPolicy(2, time.Millisecond, time.Millisecond),
	)
	require.NoError(t, m.Register(db, repo))
	require.NoError(t, m.TopoSort())

	require.NoError(t, m.Init(context.Background()))
	assert.Equal(t, 3, db.initCalled, "first attempt plus two retries")
	assert.Equal(t, 1, repo.initCalled)
}

// A retry is logged only when one follows: retry-go reports a policy's last failure as
// it does the others, and a one-pass init's only one.
func TestInitLogsARetryOnlyWhenOneFollows(t *testing.T) {
	for _, tc := range []struct{ retries, logged int }{{0, 0}, {2, 2}} {
		file := filepath.Join(t.TempDir(), "supervisor.log")
		logger := log.New(log.WithLevel(-1), log.NoStdout(), log.WithFileWriter(file, 1, 1, 1))
		db := newMockService("db")
		db.initErr = fmt.Errorf("db down")
		m := newManager(nil, WithLogger(logger), WithName("test"), WithInitPolicy(tc.retries, time.Millisecond, time.Millisecond))
		require.NoError(t, m.Register(db))
		require.NoError(t, m.TopoSort())

		require.Error(t, m.Init(context.Background()))
		logs, err := os.ReadFile(file)
		require.NoError(t, err)
		assert.Equal(t, tc.logged, strings.Count(string(logs), "retrying"), "MaxRetries %d", tc.retries)
	}
}

// dependents fail fast without burning their own retry budget
func TestInitPolicyExhausted(t *testing.T) {
	db := newMockService("db")
	db.initErr = fmt.Errorf("db down")
	repo := newMockService("repo")
	repo.deps = []common.Service{db}

	m := newTestManager(
		WithInitPolicy(2, time.Millisecond, time.Millisecond),
	)
	require.NoError(t, m.Register(db, repo))
	require.NoError(t, m.TopoSort())

	start := time.Now()
	err := m.Init(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db")
	assert.Equal(t, 3, db.initCalled, "first attempt plus two retries")
	assert.Equal(t, 0, repo.initCalled, "dependent fails fast on a dependency that gave up")
	assert.Less(t, time.Since(start), time.Second, "repo's turn must not re-wait a lost cause")

	stat := m.c.stats.snapshot("repo")
	require.NotNil(t, stat)
	assert.Error(t, stat.InitializationErr)
}

func TestInitPolicyDisabledSkipsProbes(t *testing.T) {
	db := newMockService("db")
	db.readyErr = fmt.Errorf("never ready")
	repo := newMockService("repo")
	repo.deps = []common.Service{db}

	m := newTestManager()
	require.NoError(t, m.Register(db, repo))
	require.NoError(t, m.TopoSort())

	require.NoError(t, m.Init(context.Background()))
	assert.Equal(t, 0, db.readyCalled, "no probing during a one-pass init")
	assert.Equal(t, 1, repo.initCalled)
}

func TestInitWaitHonorsCancel(t *testing.T) {
	db := newMockService("db")
	db.readyErr = fmt.Errorf("never ready")
	repo := newMockService("repo")
	repo.deps = []common.Service{db}

	m := newTestManager(
		WithInitPolicy(-1, 50*time.Millisecond, 50*time.Millisecond),
	)
	require.NoError(t, m.Register(db, repo))
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

// The policy is the boot's. A later Init — a reload's, of a process that was
// serving — makes one pass: a config that fails is the change, which no wait
// fixes, so it returns and what can start starts.
func TestALaterInitMakesOnePass(t *testing.T) {
	cfg := newMockService("cfg")
	cfg.initFn = failNTimes(2)
	other := newMockService("other")
	m := newTestManager(WithInitPolicy(-1, time.Millisecond, time.Millisecond))
	require.NoError(t, m.Register(cfg, other))
	require.NoError(t, m.TopoSort())
	require.NoError(t, m.Init(context.Background()), "the boot waits")
	assert.Equal(t, 3, cfg.initCalled)
	require.NoError(t, m.Start(context.Background()))

	// what the example's SIGHUP reload does, the file now broken
	require.NoError(t, m.Stop(context.Background()))
	cfg.initFn = func() error { return fmt.Errorf("bad tasks.yaml") }
	done := make(chan error, 1)
	go func() { done <- m.Init(context.Background()) }()
	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("the reload's Init waited on a config that fails every time")
	}
	assert.Equal(t, 4, cfg.initCalled, "one try")
	require.NoError(t, m.Start(context.Background()), "an Init failure is not reported again")
	assert.Equal(t, 1, cfg.startCalled, "the broken one started at boot alone")
	assert.Equal(t, 2, other.startCalled, "what can start starts")
	require.NoError(t, m.Stop(context.Background()))
}
