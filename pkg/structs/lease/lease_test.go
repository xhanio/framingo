package lease

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/xhanio/framingo/pkg/utils/log"
)

func TestNew(t *testing.T) {
	t.Run("with provided ID", func(t *testing.T) {
		id := "test-lease"
		duration := 1 * time.Second
		lease := New(id, duration)

		assert.Equal(t, id, lease.ID())
		assert.False(t, lease.Expired())
	})

	t.Run("with empty ID generates UUID", func(t *testing.T) {
		duration := 1 * time.Second
		lease := New("", duration)

		id := lease.ID()
		assert.NotEmpty(t, id)
		_, err := uuid.Parse(id)
		assert.NoError(t, err, "ID should be a valid UUID")
	})

	t.Run("with options", func(t *testing.T) {
		lease := New("test", 1*time.Second,
			Once(),
			UseWallTime(),
			OnExpired(func() {}),
		)

		assert.Equal(t, "test", lease.ID())
		assert.False(t, lease.Expired())

		// Options will be tested through behavior in other tests
	})
}

func TestLeaseLifecycle(t *testing.T) {
	t.Run("basic start and expiration", func(t *testing.T) {
		var expired atomic.Bool
		lease := New("test", 200*time.Millisecond, OnExpired(func() {
			expired.Store(true)
		}))

		assert.False(t, lease.Expired())

		go lease.Start()

		// Wait for expiration
		time.Sleep(300 * time.Millisecond)

		assert.True(t, expired.Load())
		assert.True(t, lease.Expired())
	})

	t.Run("cancel before expiration", func(t *testing.T) {
		var expired, cancelled atomic.Bool
		lease := New("test", 1*time.Second,
			OnExpired(func() { expired.Store(true) }),
			OnCancel(func() { cancelled.Store(true) }),
		)

		go lease.Start()

		// Cancel after a short time
		time.Sleep(100 * time.Millisecond)
		lease.Cancel()

		// Wait a bit more to ensure cancellation is processed
		time.Sleep(200 * time.Millisecond)

		assert.True(t, cancelled.Load())
		assert.False(t, expired.Load())
		assert.True(t, lease.Expired())
	})

	t.Run("multiple starts ignored", func(t *testing.T) {
		lease := New("test", 1*time.Second)

		go lease.Start()
		time.Sleep(50 * time.Millisecond) // Let it initialize

		// Second start should be ignored
		go lease.Start()

		assert.False(t, lease.Expired())
		lease.Cancel()
	})
}

func TestLeaseRefresh(t *testing.T) {
	t.Run("successful refresh", func(t *testing.T) {
		var refreshed atomic.Bool
		lease := New("test", 200*time.Millisecond, OnRefresh(func() {
			refreshed.Store(true)
		}))

		go lease.Start()
		time.Sleep(50 * time.Millisecond) // Let it initialize

		initialExpiration := lease.ExpiresAt()
		time.Sleep(100 * time.Millisecond)

		// Refresh the lease
		success := lease.Refresh(300 * time.Millisecond)
		assert.True(t, success)

		// Wait for refresh to be processed
		time.Sleep(50 * time.Millisecond)
		assert.True(t, refreshed.Load())

		newExpiration := lease.ExpiresAt()
		assert.True(t, newExpiration.After(initialExpiration))

		lease.Cancel()
	})

	t.Run("refresh after expiration fails", func(t *testing.T) {
		lease := New("test", 100*time.Millisecond)

		go lease.Start()

		// Wait for expiration
		time.Sleep(200 * time.Millisecond)

		success := lease.Refresh(1 * time.Second)
		assert.False(t, success)
		assert.True(t, lease.Expired())
	})
}

func TestLeaseExtend(t *testing.T) {
	t.Run("successful extend", func(t *testing.T) {
		var extended atomic.Bool
		lease := New("test", 200*time.Millisecond, OnExtend(func() {
			extended.Store(true)
		}))

		go lease.Start()
		time.Sleep(50 * time.Millisecond) // Let it initialize

		initialExpiration := lease.ExpiresAt()

		// Extend the lease
		success := lease.Extend(300 * time.Millisecond)
		assert.True(t, success)

		// Wait for extend to be processed
		time.Sleep(50 * time.Millisecond)
		assert.True(t, extended.Load())

		newExpiration := lease.ExpiresAt()
		assert.True(t, newExpiration.After(initialExpiration))

		lease.Cancel()
	})

	t.Run("extend after expiration fails", func(t *testing.T) {
		lease := New("test", 100*time.Millisecond)

		go lease.Start()

		// Wait for expiration
		time.Sleep(200 * time.Millisecond)

		success := lease.Extend(1 * time.Second)
		assert.False(t, success)
		assert.True(t, lease.Expired())
	})
}

func TestLeaseRenew(t *testing.T) {
	t.Run("successful renew", func(t *testing.T) {
		var renewed atomic.Bool
		lease := New("test", 200*time.Millisecond, OnRenew(func() {
			renewed.Store(true)
		}))

		go lease.Start()
		time.Sleep(50 * time.Millisecond) // Let it initialize

		newExpiration := time.Now().Add(1 * time.Second)

		// Renew the lease
		success := lease.Renew(newExpiration)
		assert.True(t, success)

		// Wait for renew to be processed
		time.Sleep(50 * time.Millisecond)
		assert.True(t, renewed.Load())

		actualExpiration := lease.ExpiresAt()
		assert.WithinDuration(t, newExpiration, actualExpiration, 10*time.Millisecond)

		lease.Cancel()
	})

	t.Run("renew after expiration fails", func(t *testing.T) {
		lease := New("test", 100*time.Millisecond)

		go lease.Start()

		// Wait for expiration
		time.Sleep(200 * time.Millisecond)

		newExpiration := time.Now().Add(1 * time.Second)
		success := lease.Renew(newExpiration)
		assert.False(t, success)
		assert.True(t, lease.Expired())
	})
}

func TestLeaseHooks(t *testing.T) {
	t.Run("all hooks called appropriately", func(t *testing.T) {
		var (
			refreshCalled atomic.Bool
			extendCalled  atomic.Bool
			renewCalled   atomic.Bool
			cancelCalled  atomic.Bool
			expiredCalled atomic.Bool
		)

		lease := New("test", 500*time.Millisecond)

		// Add hooks after creation
		lease.OnRefresh(func() { refreshCalled.Store(true) })
		lease.OnExtend(func() { extendCalled.Store(true) })
		lease.OnRenew(func() { renewCalled.Store(true) })
		lease.OnCancel(func() { cancelCalled.Store(true) })
		lease.OnExpired(func() { expiredCalled.Store(true) })

		go lease.Start()
		time.Sleep(50 * time.Millisecond) // Let it initialize

		// Test refresh
		lease.Refresh(500 * time.Millisecond)
		time.Sleep(50 * time.Millisecond)
		assert.True(t, refreshCalled.Load())

		// Test extend
		lease.Extend(200 * time.Millisecond)
		time.Sleep(50 * time.Millisecond)
		assert.True(t, extendCalled.Load())

		// Test renew
		lease.Renew(time.Now().Add(200 * time.Millisecond))
		time.Sleep(50 * time.Millisecond)
		assert.True(t, renewCalled.Load())

		// Test cancel
		lease.Cancel()
		time.Sleep(100 * time.Millisecond)
		assert.True(t, cancelCalled.Load())
		assert.False(t, expiredCalled.Load()) // Should not expire when cancelled
	})

	t.Run("multiple hooks of same type", func(t *testing.T) {
		var count1, count2 atomic.Int32

		lease := New("test", 200*time.Millisecond)
		lease.OnExpired(func() { count1.Add(1) })
		lease.OnExpired(func() { count2.Add(1) })

		go lease.Start()

		// Wait for expiration
		time.Sleep(300 * time.Millisecond)

		assert.Equal(t, int32(1), count1.Load())
		assert.Equal(t, int32(1), count2.Load())
	})
}

func TestLeaseOptions(t *testing.T) {
	t.Run("Once option prevents restart", func(t *testing.T) {
		var expiredCount atomic.Int32
		lease := New("test", 100*time.Millisecond,
			Once(),
			OnExpired(func() { expiredCount.Add(1) }),
		)

		// First start
		go lease.Start()
		time.Sleep(200 * time.Millisecond)
		assert.Equal(t, int32(1), expiredCount.Load())
		assert.True(t, lease.Expired())

		// Second start should be ignored
		go lease.Start()
		time.Sleep(200 * time.Millisecond)
		assert.Equal(t, int32(1), expiredCount.Load()) // Should not increment
	})

	t.Run("UseWallTime option", func(t *testing.T) {
		lease := New("test", 1*time.Second, UseWallTime())

		// UseWallTime behavior will be verified through timing tests
		assert.Equal(t, "test", lease.ID())
	})

	t.Run("WithLogger option", func(t *testing.T) {
		logger := log.New()
		lease := New("test", 1*time.Second, WithLogger(logger))

		// Logger is set internally, verified by successful creation
		assert.Equal(t, "test", lease.ID())
	})

	t.Run("hook options during creation", func(t *testing.T) {
		var called atomic.Bool
		lease := New("test", 100*time.Millisecond, OnExpired(func() {
			called.Store(true)
		}))

		go lease.Start()
		time.Sleep(200 * time.Millisecond)

		assert.True(t, called.Load())
	})
}

func TestLeaseThreadSafety(t *testing.T) {
	t.Run("concurrent operations", func(t *testing.T) {
		lease := New("test", 1*time.Second)

		go lease.Start()
		time.Sleep(50 * time.Millisecond) // Let it initialize

		var wg sync.WaitGroup
		const numGoroutines = 100

		// Test concurrent reads
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				lease.ID()
				lease.Expired()
				lease.ExpiresAt()
			}()
		}

		// Test concurrent operations
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				switch i % 4 {
				case 0:
					lease.Refresh(100 * time.Millisecond)
				case 1:
					lease.Extend(100 * time.Millisecond)
				case 2:
					lease.Renew(time.Now().Add(500 * time.Millisecond))
				case 3:
					lease.OnExpired(func() {})
				}
			}(i)
		}

		wg.Wait()
		lease.Cancel()
	})
}

func TestLeaseEdgeCases(t *testing.T) {
	t.Run("operations on cancelled lease", func(t *testing.T) {
		lease := New("test", 1*time.Second)

		go lease.Start()
		time.Sleep(50 * time.Millisecond) // Let it initialize

		lease.Cancel()
		time.Sleep(100 * time.Millisecond) // Let cancellation process

		// All operations should return false on cancelled lease
		assert.False(t, lease.Refresh(1*time.Second))
		assert.False(t, lease.Extend(1*time.Second))
		assert.False(t, lease.Renew(time.Now().Add(1*time.Second)))
		assert.True(t, lease.Expired())
	})

	t.Run("multiple cancellations", func(t *testing.T) {
		var cancelCount atomic.Int32
		lease := New("test", 1*time.Second, OnCancel(func() {
			cancelCount.Add(1)
		}))

		go lease.Start()
		time.Sleep(50 * time.Millisecond) // Let it initialize

		// Multiple cancel calls
		lease.Cancel()
		lease.Cancel()
		lease.Cancel()

		time.Sleep(100 * time.Millisecond)

		// Should only be called once
		assert.Equal(t, int32(1), cancelCount.Load())
	})

	t.Run("very short duration", func(t *testing.T) {
		var expired atomic.Bool
		lease := New("test", 1*time.Millisecond, OnExpired(func() {
			expired.Store(true)
		}))

		go lease.Start()

		// Should expire very quickly
		time.Sleep(200 * time.Millisecond)

		assert.True(t, expired.Load())
		assert.True(t, lease.Expired())
	})

	t.Run("zero duration", func(t *testing.T) {
		var expired atomic.Bool
		lease := New("test", 0, OnExpired(func() {
			expired.Store(true)
		}))

		go lease.Start()

		// Should expire immediately on first tick
		time.Sleep(200 * time.Millisecond)

		assert.True(t, expired.Load())
		assert.True(t, lease.Expired())
	})
}

func TestLeaseExpiresAtAccuracy(t *testing.T) {
	t.Run("expiration time accuracy", func(t *testing.T) {
		duration := 500 * time.Millisecond
		beforeStart := time.Now()
		lease := New("test", duration)

		go lease.Start()
		time.Sleep(10 * time.Millisecond) // Let it initialize
		afterStart := time.Now()

		expiresAt := lease.ExpiresAt()
		expectedEarliest := beforeStart.Add(duration)
		expectedLatest := afterStart.Add(duration)

		assert.True(t, expiresAt.After(expectedEarliest) || expiresAt.Equal(expectedEarliest))
		assert.True(t, expiresAt.Before(expectedLatest) || expiresAt.Equal(expectedLatest))

		lease.Cancel()
	})
}

func BenchmarkLeaseOperations(b *testing.B) {
	b.Run("New", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			lease := New("test", 1*time.Second)
			_ = lease
		}
	})

	b.Run("ID", func(b *testing.B) {
		lease := New("test", 1*time.Second)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = lease.ID()
		}
	})

	b.Run("Expired", func(b *testing.B) {
		lease := New("test", 1*time.Second)
		go lease.Start()
		time.Sleep(10 * time.Millisecond) // Let it initialize

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = lease.Expired()
		}

		lease.Cancel()
	})

	b.Run("Refresh", func(b *testing.B) {
		lease := New("test", 1*time.Second)
		go lease.Start()
		time.Sleep(10 * time.Millisecond) // Let it initialize

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			lease.Refresh(1 * time.Second)
		}

		lease.Cancel()
	})
}
