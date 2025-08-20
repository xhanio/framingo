package buffer

import (
	"sync"
	"testing"
)

func TestNewPool(t *testing.T) {
	pool := NewPool[byte]()
	if pool == nil {
		t.Error("NewPool() should return a non-nil pool")
	}

	// Check initial stats
	gets, puts, hits, creates, hitRate := pool.Stats()
	if gets != 0 || puts != 0 || hits != 0 || creates != 0 || hitRate != 0 {
		t.Errorf("New pool should have zero stats, got gets=%d, puts=%d, hits=%d, creates=%d, hitRate=%f",
			gets, puts, hits, creates, hitRate)
	}
}

func TestPoolGet(t *testing.T) {
	tests := []struct {
		name          string
		required      int
		expectedCap   int
		shouldUsePool bool
	}{
		{
			name:          "get small buffer (1KB)",
			required:      500,
			expectedCap:   1024,
			shouldUsePool: true,
		},
		{
			name:          "get exact size (4KB)",
			required:      4096,
			expectedCap:   4096,
			shouldUsePool: true,
		},
		{
			name:          "get medium buffer (16KB)",
			required:      10000,
			expectedCap:   16384,
			shouldUsePool: true,
		},
		{
			name:          "get large buffer (1MB)",
			required:      500000,
			expectedCap:   1048576,
			shouldUsePool: true,
		},
		{
			name:          "get oversized buffer",
			required:      2000000, // 2MB, larger than largest pool size
			expectedCap:   2000000,
			shouldUsePool: false,
		},
		{
			name:          "get zero size",
			required:      0,
			expectedCap:   1024, // should get smallest pool size
			shouldUsePool: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := NewPool[byte]()
			buffer := pool.Get(tt.required)

			if len(buffer) != 0 {
				t.Errorf("Buffer length should be 0, got %d", len(buffer))
			}

			if cap(buffer) < tt.required {
				t.Errorf("Buffer capacity should be at least %d, got %d", tt.required, cap(buffer))
			}

			if tt.shouldUsePool && cap(buffer) != tt.expectedCap {
				t.Errorf("Expected capacity %d, got %d", tt.expectedCap, cap(buffer))
			}

			// Check stats
			gets, _, hits, creates, _ := pool.Stats()
			if gets != 1 {
				t.Errorf("Expected 1 get, got %d", gets)
			}
			if tt.shouldUsePool {
				if creates != 1 {
					t.Errorf("Expected 1 create for first get, got %d", creates)
				}
				if hits != 1 {
					t.Errorf("Expected 1 hit for pool get, got %d", hits)
				}
			}
		})
	}
}

func TestPoolPut(t *testing.T) {
	pool := NewPool[byte]()

	// Test putting nil buffer
	pool.Put(nil)
	_, puts, _, _, _ := pool.Stats()
	if puts != 0 {
		t.Errorf("Putting nil should not increment puts, got %d", puts)
	}

	// Test putting buffer with pool size
	buffer := make([]byte, 100, 1024) // capacity matches pool size
	pool.Put(buffer)
	_, puts, _, _, _ = pool.Stats()
	if puts != 1 {
		t.Errorf("Expected 1 put, got %d", puts)
	}

	// Test putting buffer with non-pool size
	buffer2 := make([]byte, 100, 2000) // capacity doesn't match any pool size
	pool.Put(buffer2)
	_, puts, _, _, _ = pool.Stats()
	if puts != 2 {
		t.Errorf("Expected 2 puts, got %d", puts)
	}
}

func TestPoolGetPutCycle(t *testing.T) {
	pool := NewPool[byte]()

	// Get a buffer
	buffer1 := pool.Get(1000)
	if cap(buffer1) != 1024 {
		t.Errorf("Expected capacity 1024, got %d", cap(buffer1))
	}

	// Use the buffer
	buffer1 = append(buffer1, []byte("hello world")...)

	// Put it back
	pool.Put(buffer1)

	// Get another buffer of same size - should reuse
	buffer2 := pool.Get(1000)
	if cap(buffer2) != 1024 {
		t.Errorf("Expected capacity 1024, got %d", cap(buffer2))
	}

	// Buffer should be reset to length 0
	if len(buffer2) != 0 {
		t.Errorf("Reused buffer should have length 0, got %d", len(buffer2))
	}

	// Check stats
	gets, puts, hits, creates, hitRate := pool.Stats()
	if gets != 2 {
		t.Errorf("Expected 2 gets, got %d", gets)
	}
	if puts != 1 {
		t.Errorf("Expected 1 put, got %d", puts)
	}
	if hits != 2 {
		t.Errorf("Expected 2 hits, got %d", hits)
	}
	if creates != 1 {
		t.Errorf("Expected 1 create (first get), got %d", creates)
	}
	if hitRate != 100.0 {
		t.Errorf("Expected hit rate 100%%, got %f%%", hitRate)
	}
}

func TestPoolStats(t *testing.T) {
	pool := NewPool[byte]()

	// Initial stats
	gets, puts, hits, creates, hitRate := pool.Stats()
	if gets != 0 || puts != 0 || hits != 0 || creates != 0 || hitRate != 0 {
		t.Error("Initial stats should all be zero")
	}

	// Get some buffers
	buffer1 := pool.Get(1000)
	buffer2 := pool.Get(2000)
	buffer3 := pool.Get(10000000) // oversized

	gets, puts, hits, creates, hitRate = pool.Stats()
	if gets != 3 {
		t.Errorf("Expected 3 gets, got %d", gets)
	}
	if creates != 3 { // all first-time gets
		t.Errorf("Expected 3 creates, got %d", creates)
	}
	if hits != 2 { // first two gets hit the pool
		t.Errorf("Expected 2 hits, got %d", hits)
	}
	expectedHitRate1 := float64(2) / float64(3) * 100 // 66.67%
	if hitRate != expectedHitRate1 {
		t.Errorf("Expected hit rate %f%%, got %f%%", expectedHitRate1, hitRate)
	}

	// Put and get again
	pool.Put(buffer1)
	pool.Put(buffer2)
	pool.Put(buffer3) // this won't be pooled due to size

	buffer4 := pool.Get(1000) // should hit
	buffer5 := pool.Get(2000) // should hit

	gets, puts, hits, creates, hitRate = pool.Stats()
	if gets != 5 {
		t.Errorf("Expected 5 gets, got %d", gets)
	}
	if puts != 3 {
		t.Errorf("Expected 3 puts, got %d", puts)
	}
	if hits != 4 {
		t.Errorf("Expected 4 hits, got %d", hits)
	}
	if creates != 3 {
		t.Errorf("Expected 3 creates, got %d", creates)
	}
	expectedHitRate := float64(4) / float64(5) * 100 // 80%
	if hitRate != expectedHitRate {
		t.Errorf("Expected hit rate %f%%, got %f%%", expectedHitRate, hitRate)
	}

	// Use the returned buffers to avoid compiler optimization
	_ = buffer4
	_ = buffer5
}

func TestPoolConcurrentAccess(t *testing.T) {
	pool := NewPool[byte]()
	var wg sync.WaitGroup

	// Number of goroutines
	numGoroutines := 100
	numOperations := 10

	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()

			buffers := make([][]byte, numOperations)

			// Get buffers
			for j := 0; j < numOperations; j++ {
				buffers[j] = pool.Get(1000 + j*100) // varying sizes
				// Write some data
				buffers[j] = append(buffers[j], []byte("test data")...)
			}

			// Put buffers back
			for j := 0; j < numOperations; j++ {
				pool.Put(buffers[j])
			}
		}()
	}

	wg.Wait()

	// Check that all operations were recorded
	gets, puts, hits, _, hitRate := pool.Stats()
	expectedOps := int64(numGoroutines * numOperations)

	if gets != expectedOps {
		t.Errorf("Expected %d gets, got %d", expectedOps, gets)
	}
	if puts != expectedOps {
		t.Errorf("Expected %d puts, got %d", expectedOps, puts)
	}

	// Should have some hits due to reuse
	if hits == 0 {
		t.Error("Expected some hits due to concurrent reuse")
	}

	if hitRate < 0 || hitRate > 100 {
		t.Errorf("Hit rate should be between 0-100%%, got %f%%", hitRate)
	}
}

func TestPoolFindBestSize(t *testing.T) {
	pool := newPool[byte]()

	tests := []struct {
		required int
		expected int
	}{
		{0, 1024},
		{1, 1024},
		{1024, 1024},
		{1025, 4096},
		{4096, 4096},
		{4097, 16384},
		{16384, 16384},
		{16385, 65536},
		{65536, 65536},
		{65537, 262144},
		{262144, 262144},
		{262145, 1048576},
		{1048576, 1048576},
		{1048577, 0}, // no suitable size
		{2000000, 0}, // too large
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := pool.findBestSize(tt.required)
			if result != tt.expected {
				t.Errorf("findBestSize(%d) = %d, want %d", tt.required, result, tt.expected)
			}
		})
	}
}

func TestPoolGenericTypes(t *testing.T) {
	// Test with int slice
	intPool := NewPool[int]()
	intBuffer := intPool.Get(10)

	if len(intBuffer) != 0 {
		t.Errorf("Int buffer length should be 0, got %d", len(intBuffer))
	}
	if cap(intBuffer) < 10 {
		t.Errorf("Int buffer capacity should be at least 10, got %d", cap(intBuffer))
	}

	// Use the buffer
	intBuffer = append(intBuffer, 1, 2, 3)
	intPool.Put(intBuffer)

	// Test with string slice
	stringPool := NewPool[string]()
	stringBuffer := stringPool.Get(5)

	if len(stringBuffer) != 0 {
		t.Errorf("String buffer length should be 0, got %d", len(stringBuffer))
	}
	if cap(stringBuffer) < 5 {
		t.Errorf("String buffer capacity should be at least 5, got %d", cap(stringBuffer))
	}

	stringBuffer = append(stringBuffer, "hello", "world")
	stringPool.Put(stringBuffer)

	// Verify stats work for different types
	gets, puts, _, _, _ := intPool.Stats()
	if gets != 1 || puts != 1 {
		t.Errorf("Int pool stats: expected gets=1, puts=1, got gets=%d, puts=%d", gets, puts)
	}

	gets, puts, _, _, _ = stringPool.Stats()
	if gets != 1 || puts != 1 {
		t.Errorf("String pool stats: expected gets=1, puts=1, got gets=%d, puts=%d", gets, puts)
	}
}
