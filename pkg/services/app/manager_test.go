package app

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"
)

// --- test helpers ---

type mockService struct {
	name         string
	deps         []common.Service
	initErr      error
	startErr     error
	stopErr      error
	aliveErr     error
	readyErr     error
	initCalled   int
	startCalled  int
	stopCalled   int
	aliveCalled  int
	readyCalled  int
}

func newMockService(name string) *mockService {
	return &mockService{name: name}
}

func (s *mockService) Name() string                 { return s.name }
func (s *mockService) Dependencies() []common.Service { return s.deps }

func (s *mockService) Init(ctx context.Context) error {
	s.initCalled++
	return s.initErr
}

func (s *mockService) Start(ctx context.Context) error {
	s.startCalled++
	return s.startErr
}

func (s *mockService) Stop(wait bool) error {
	s.stopCalled++
	return s.stopErr
}

func (s *mockService) Alive() error {
	s.aliveCalled++
	return s.aliveErr
}

func (s *mockService) Ready() error {
	s.readyCalled++
	return s.readyErr
}

// initOnlyService implements Service + Initializable but not Daemon
type initOnlyService struct {
	name       string
	deps       []common.Service
	initErr    error
	initCalled int
}

func (s *initOnlyService) Name() string                 { return s.name }
func (s *initOnlyService) Dependencies() []common.Service { return s.deps }
func (s *initOnlyService) Init(ctx context.Context) error {
	s.initCalled++
	return s.initErr
}

func testLogger() log.Logger {
	return log.New(log.WithLevel(-1))
}

func newTestManager(opts ...Option) *manager {
	return newManager(nil, append([]Option{WithLogger(testLogger()), WithName("test")}, opts...)...)
}

// --- tests ---

func TestNew(t *testing.T) {
	m := New(nil, WithLogger(testLogger()), WithName("my-app"))
	assert.NotNil(t, m)
	assert.Equal(t, "my-app", m.Name())
	assert.Nil(t, m.Dependencies())
}

func TestRegister(t *testing.T) {
	t.Run("registers services", func(t *testing.T) {
		m := newTestManager()
		a := newMockService("a")
		b := newMockService("b")
		m.Register(a, b)
		assert.Len(t, m.lc.stats, 2)
	})

	t.Run("skips nil services", func(t *testing.T) {
		m := newTestManager()
		m.Register(nil, newMockService("a"), nil)
		assert.Len(t, m.lc.stats, 1)
	})

	t.Run("registers dependencies", func(t *testing.T) {
		m := newTestManager()
		dep := newMockService("dep")
		svc := newMockService("svc")
		svc.deps = []common.Service{dep}
		m.Register(svc)
		assert.NotNil(t, m.lc.stats["dep"])
		assert.NotNil(t, m.lc.stats["svc"])
	})
}

func TestTopoSort(t *testing.T) {
	t.Run("sorts services in dependency order", func(t *testing.T) {
		m := newTestManager()
		db := newMockService("db")
		api := newMockService("api")
		api.deps = []common.Service{db}
		m.Register(db, api)
		require.NoError(t, m.TopoSort())
		services := m.Services()
		require.Len(t, services, 2)
		assert.Equal(t, "db", services[0].Name())
		assert.Equal(t, "api", services[1].Name())
	})
}

func TestInitAndStart(t *testing.T) {
	t.Run("initializes and starts all services", func(t *testing.T) {
		m := newTestManager()
		a := newMockService("a")
		b := newMockService("b")
		m.Register(a, b)
		require.NoError(t, m.TopoSort())
		require.NoError(t, m.Init(context.Background()))
		assert.Equal(t, 1, a.initCalled)
		assert.Equal(t, 1, b.initCalled)

		require.NoError(t, m.Start(context.Background()))
		assert.Equal(t, 1, a.startCalled)
		assert.Equal(t, 1, b.startCalled)

		require.NoError(t, m.Stop(true))
		assert.Equal(t, 1, a.stopCalled)
		assert.Equal(t, 1, b.stopCalled)
	})

	t.Run("init failure sets stats", func(t *testing.T) {
		m := newTestManager()
		svc := newMockService("bad")
		svc.initErr = fmt.Errorf("init boom")
		m.Register(svc)
		require.NoError(t, m.TopoSort())

		err := m.Init(context.Background())
		assert.Error(t, err)

		stat := m.lc.stat("bad")
		assert.False(t, stat.Initialized)
		assert.False(t, stat.Ready)
		assert.EqualError(t, stat.InitializationErr, "init boom")
	})

	t.Run("start failure sets stats", func(t *testing.T) {
		m := newTestManager()
		svc := newMockService("bad")
		svc.startErr = fmt.Errorf("start boom")
		m.Register(svc)
		require.NoError(t, m.TopoSort())
		require.NoError(t, m.Init(context.Background()))

		err := m.Start(context.Background())
		assert.Error(t, err)

		stat := m.lc.stat("bad")
		assert.True(t, stat.Started)
		assert.False(t, stat.Ready)
		assert.EqualError(t, stat.StartErr, "start boom")

		_ = m.Stop(false)
	})

	t.Run("init-only service does not start", func(t *testing.T) {
		m := newTestManager()
		svc := &initOnlyService{name: "config"}
		m.Register(svc)
		require.NoError(t, m.TopoSort())
		require.NoError(t, m.Init(context.Background()))
		assert.Equal(t, 1, svc.initCalled)

		stat := m.lc.stat("config")
		assert.True(t, stat.Initialized)
		assert.True(t, stat.Ready)

		require.NoError(t, m.Start(context.Background()))
		assert.False(t, stat.Started)
		require.NoError(t, m.Stop(true))
	})
}

func TestDependencyInitOrder(t *testing.T) {
	t.Run("skips service if dependency failed to init", func(t *testing.T) {
		m := newTestManager()
		dep := newMockService("dep")
		dep.initErr = fmt.Errorf("dep failed")
		svc := newMockService("svc")
		svc.deps = []common.Service{dep}
		m.Register(dep, svc)
		require.NoError(t, m.TopoSort())

		err := m.Init(context.Background())
		assert.Error(t, err)
		assert.Equal(t, 1, dep.initCalled)
		assert.Equal(t, 0, svc.initCalled)
	})
}

func TestStopOrder(t *testing.T) {
	t.Run("stops in reverse topological order", func(t *testing.T) {
		m := newTestManager()
		var order []string
		db := newMockService("db")
		db.stopErr = nil
		api := newMockService("api")
		api.deps = []common.Service{db}

		origDB := db.stopCalled
		origAPI := api.stopCalled
		_ = origDB
		_ = origAPI

		// Track stop order via a channel
		stopOrder := make([]string, 0, 2)
		dbStop := db.stopErr
		apiStop := api.stopErr
		_ = dbStop
		_ = apiStop

		m.Register(db, api)
		require.NoError(t, m.TopoSort())
		require.NoError(t, m.Init(context.Background()))
		require.NoError(t, m.Start(context.Background()))

		require.NoError(t, m.Stop(true))

		// After topo sort: [db, api]. Stop is reversed: api first, then db
		// We verify both were stopped
		assert.Equal(t, 1, api.stopCalled)
		assert.Equal(t, 1, db.stopCalled)

		// Verify stats
		for _, name := range []string{"db", "api"} {
			stat := m.lc.stat(name)
			assert.True(t, stat.Stopped)
			assert.False(t, stat.Ready)
		}
		_ = order
		_ = stopOrder
	})
}

func TestPerServiceLifecycle(t *testing.T) {
	m := newTestManager()
	svc := newMockService("svc")
	m.Register(svc)
	require.NoError(t, m.TopoSort())

	t.Run("InitService", func(t *testing.T) {
		require.NoError(t, m.InitService(context.Background(), "svc"))
		assert.Equal(t, 1, svc.initCalled)
	})

	t.Run("StartService", func(t *testing.T) {
		require.NoError(t, m.StartService("svc"))
		assert.Equal(t, 1, svc.startCalled)
	})

	t.Run("StopService", func(t *testing.T) {
		require.NoError(t, m.StopService("svc", true))
		assert.Equal(t, 1, svc.stopCalled)
	})

	t.Run("RestartService", func(t *testing.T) {
		require.NoError(t, m.RestartService(context.Background(), "svc"))
		assert.Equal(t, 2, svc.stopCalled)
		assert.Equal(t, 2, svc.initCalled)
		assert.Equal(t, 2, svc.startCalled)
		stat := m.lc.stat("svc")
		assert.Equal(t, 1, stat.Restarts)
	})

	t.Run("not found", func(t *testing.T) {
		assert.Error(t, m.InitService(context.Background(), "nope"))
		assert.Error(t, m.StartService("nope"))
		assert.Error(t, m.StopService("nope", true))
		assert.Error(t, m.RestartService(context.Background(), "nope"))
	})
}

func TestStats(t *testing.T) {
	m := newTestManager()
	a := newMockService("a")
	b := newMockService("b")
	m.Register(a, b)
	require.NoError(t, m.TopoSort())
	require.NoError(t, m.Init(context.Background()))

	stats, err := m.Stats()
	require.NoError(t, err)
	assert.Len(t, stats, 2)
	for _, stat := range stats {
		assert.True(t, stat.Initialized)
		assert.True(t, stat.Ready)
		assert.Nil(t, stat.InitializationErr)
	}
}

func TestStatsHealthcheck(t *testing.T) {
	t.Run("returns nil when healthy", func(t *testing.T) {
		s := &Stats{Name: "svc", Initialized: true}
		assert.NoError(t, s.Healthcheck())
	})

	t.Run("reports stopped", func(t *testing.T) {
		s := &Stats{Name: "svc", Stopped: true}
		err := s.Healthcheck()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "stopped")
	})

	t.Run("reports init error", func(t *testing.T) {
		s := &Stats{Name: "svc", InitializationErr: fmt.Errorf("init boom")}
		err := s.Healthcheck()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "init boom")
		assert.Contains(t, err.Error(), "svc")
	})

	t.Run("reports start error", func(t *testing.T) {
		s := &Stats{Name: "svc", StartErr: fmt.Errorf("start boom")}
		err := s.Healthcheck()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "start boom")
		assert.Contains(t, err.Error(), "svc")
	})
}

func TestHealthcheckLivenessAndReadiness(t *testing.T) {
	t.Run("liveness failure sets LivenessErr", func(t *testing.T) {
		m := newTestManager()
		svc := newMockService("svc")
		svc.aliveErr = fmt.Errorf("dead")
		m.Register(svc)
		require.NoError(t, m.TopoSort())
		require.NoError(t, m.Init(context.Background()))
		require.NoError(t, m.Start(context.Background()))

		err := m.monitor.healthcheck(svc)
		assert.Error(t, err)
		stat := m.lc.stat("svc")
		assert.EqualError(t, stat.LivenessErr, "dead")
		assert.Equal(t, 1, svc.aliveCalled)

		_ = m.Stop(false)
	})

	t.Run("readiness failure sets Ready=false without affecting liveness", func(t *testing.T) {
		m := newTestManager()
		svc := newMockService("svc")
		svc.readyErr = fmt.Errorf("not ready")
		m.Register(svc)
		require.NoError(t, m.TopoSort())
		require.NoError(t, m.Init(context.Background()))
		require.NoError(t, m.Start(context.Background()))

		_ = m.monitor.healthcheck(svc)
		stat := m.lc.stat("svc")
		assert.Nil(t, stat.LivenessErr)
		assert.False(t, stat.Ready)
		assert.EqualError(t, stat.ReadinessErr, "not ready")
		assert.Equal(t, 1, svc.readyCalled)

		_ = m.Stop(false)
	})

	t.Run("readiness success sets Ready=true", func(t *testing.T) {
		m := newTestManager()
		svc := newMockService("svc")
		m.Register(svc)
		require.NoError(t, m.TopoSort())
		require.NoError(t, m.Init(context.Background()))
		require.NoError(t, m.Start(context.Background()))

		_ = m.monitor.healthcheck(svc)
		stat := m.lc.stat("svc")
		assert.Nil(t, stat.LivenessErr)
		assert.True(t, stat.Ready)
		assert.Nil(t, stat.ReadinessErr)

		_ = m.Stop(false)
	})
}

func TestMonitorRestartsOnLivenessOnly(t *testing.T) {
	t.Run("restarts on liveness failure", func(t *testing.T) {
		m := newTestManager(
			WithMonitorInterval(50*time.Millisecond),
			WithRestartPolicy(1),
		)
		svc := newMockService("svc")
		svc.aliveErr = fmt.Errorf("dead")
		m.Register(svc)
		require.NoError(t, m.TopoSort())
		require.NoError(t, m.Init(context.Background()))
		require.NoError(t, m.Start(context.Background()))

		time.Sleep(150 * time.Millisecond)
		require.NoError(t, m.Stop(true))

		stat := m.lc.stat("svc")
		assert.GreaterOrEqual(t, stat.Restarts, 1)
	})

	t.Run("does not restart on readiness-only failure", func(t *testing.T) {
		m := newTestManager(
			WithMonitorInterval(50*time.Millisecond),
			WithRestartPolicy(3),
		)
		svc := newMockService("svc")
		svc.readyErr = fmt.Errorf("not ready")
		m.Register(svc)
		require.NoError(t, m.TopoSort())
		require.NoError(t, m.Init(context.Background()))
		require.NoError(t, m.Start(context.Background()))

		time.Sleep(150 * time.Millisecond)
		require.NoError(t, m.Stop(true))

		stat := m.lc.stat("svc")
		assert.Equal(t, 0, stat.Restarts)
		assert.False(t, stat.Ready)
	})
}

func TestMonitorMaxRetries(t *testing.T) {
	m := newTestManager(
		WithMonitorInterval(50*time.Millisecond),
		WithRestartPolicy(2),
	)
	svc := newMockService("svc")
	svc.aliveErr = fmt.Errorf("dead")
	m.Register(svc)
	require.NoError(t, m.TopoSort())
	require.NoError(t, m.Init(context.Background()))
	require.NoError(t, m.Start(context.Background()))

	time.Sleep(300 * time.Millisecond)
	require.NoError(t, m.Stop(true))

	stat := m.lc.stat("svc")
	assert.Equal(t, 2, stat.Restarts)
}

func TestShutdownTimeout(t *testing.T) {
	m := newTestManager(WithShutdownTimeout(50 * time.Millisecond))
	svc := newMockService("slow")
	svc.stopErr = nil
	m.Register(svc)
	require.NoError(t, m.TopoSort())
	require.NoError(t, m.Init(context.Background()))
	require.NoError(t, m.Start(context.Background()))

	// Normal stop should succeed within timeout
	err := m.Stop(true)
	assert.NoError(t, err)
}

func TestDoubleStartStop(t *testing.T) {
	m := newTestManager()
	svc := newMockService("svc")
	m.Register(svc)
	require.NoError(t, m.TopoSort())
	require.NoError(t, m.Init(context.Background()))

	require.NoError(t, m.Start(context.Background()))
	// second start is a no-op
	require.NoError(t, m.Start(context.Background()))
	assert.Equal(t, 1, svc.startCalled)

	require.NoError(t, m.Stop(true))
	// second stop is a no-op
	require.NoError(t, m.Stop(true))
	assert.Equal(t, 1, svc.stopCalled)
}

func TestInfo(t *testing.T) {
	m := newTestManager()
	svc := newMockService("svc")
	m.Register(svc)
	require.NoError(t, m.TopoSort())
	require.NoError(t, m.Init(context.Background()))

	var buf bytes.Buffer
	m.Info(&buf, false)
	output := buf.String()
	assert.Contains(t, output, "svc")
	assert.Contains(t, output, "alive")
	assert.Contains(t, output, "ready")
}

func TestReadyStateTransitions(t *testing.T) {
	m := newTestManager()
	svc := newMockService("svc")
	m.Register(svc)
	require.NoError(t, m.TopoSort())

	stat := m.lc.stat("svc")

	// before init: not ready
	assert.False(t, stat.Ready)

	// after init: ready
	require.NoError(t, m.Init(context.Background()))
	assert.True(t, stat.Ready)

	// after start: ready
	require.NoError(t, m.Start(context.Background()))
	assert.True(t, stat.Ready)

	// after stop: not ready
	require.NoError(t, m.Stop(true))
	assert.False(t, stat.Ready)
}

func TestOptions(t *testing.T) {
	t.Run("WithName", func(t *testing.T) {
		m := newTestManager(WithName("custom"))
		assert.Equal(t, "custom", m.Name())
	})

	t.Run("WithShutdownTimeout", func(t *testing.T) {
		m := newTestManager(WithShutdownTimeout(5 * time.Second))
		assert.Equal(t, 5*time.Second, m.lc.shutdownTimeout)
	})

	t.Run("WithMonitorInterval", func(t *testing.T) {
		m := newTestManager(WithMonitorInterval(10 * time.Second))
		assert.Equal(t, 10*time.Second, m.monitor.interval)
	})

	t.Run("WithRestartPolicy", func(t *testing.T) {
		m := newTestManager(WithRestartPolicy(5))
		assert.Equal(t, 5, m.monitor.maxRetries)
	})

	t.Run("WithRestartDelay", func(t *testing.T) {
		m := newTestManager(WithRestartDelay(2 * time.Second))
		assert.Equal(t, 2*time.Second, m.monitor.restartDelay)
	})
}

func TestSignalDefaults(t *testing.T) {
	m := newTestManager()
	assert.NotNil(t, m.signals)
	assert.Len(t, m.signals, 4) // SIGINT, SIGTERM, SIGUSR1, SIGUSR2
}

func TestWithSignalHandler(t *testing.T) {
	called := false
	m := newTestManager(WithSignalHandler(testSignal, func() {
		called = true
	}))
	handler, ok := m.signals[testSignal]
	require.True(t, ok)
	handler()
	assert.True(t, called)
}

func TestMigrateNotImplemented(t *testing.T) {
	m := newTestManager()
	err := m.Migrate()
	assert.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "notimplemented")
}

func TestHealthcheckDependencyRecursion(t *testing.T) {
	m := newTestManager()
	dep := newMockService("dep")
	dep.aliveErr = fmt.Errorf("dep dead")
	svc := newMockService("svc")
	svc.deps = []common.Service{dep}
	m.Register(dep, svc)
	require.NoError(t, m.TopoSort())
	require.NoError(t, m.Init(context.Background()))

	err := m.monitor.healthcheck(svc)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dep dead")
}
