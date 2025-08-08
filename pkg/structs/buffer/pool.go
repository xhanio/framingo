package buffer

import "sync"

type pool[T any] struct {
	pools map[int]*sync.Pool // pools for different sizes
	sizes []int              // supported size list
	stats struct {
		sync.RWMutex
		gets    int64 // get count
		puts    int64 // put count
		hits    int64 // hit count
		creates int64 // create count
	}
}

// NewPool creates a new generic buffer pool instance
func NewPool[T any]() PoolG[T] {
	return newPool[T]()
}

// newPool creates a buffer pool
func newPool[T any]() *pool[T] {
	// predefined buffer sizes: 1KB, 4KB, 16KB, 64KB, 256KB, 1MB
	sizes := []int{1024, 4096, 16384, 65536, 262144, 1048576}

	p := &pool[T]{
		pools: make(map[int]*sync.Pool),
		sizes: sizes,
	}

	// create a pool for each size
	for _, size := range sizes {
		p.pools[size] = &sync.Pool{
			New: func() any {
				p.stats.Lock()
				p.stats.creates++
				p.stats.Unlock()
				return make([]T, 0, size)
			},
		}
	}

	return p
}

// Get retrieves a buffer of the specified size
func (p *pool[T]) Get(required int) []T {
	p.stats.Lock()
	p.stats.gets++
	p.stats.Unlock()

	targetSize := p.findBestSize(required)
	if targetSize == 0 {
		p.stats.Lock()
		p.stats.creates++
		p.stats.Unlock()
		return make([]T, 0, required)
	}

	buffer, ok := p.pools[targetSize].Get().([]T)
	if !ok {
		p.stats.Lock()
		p.stats.creates++
		p.stats.Unlock()
		return make([]T, 0, required)
	}

	p.stats.Lock()
	p.stats.hits++
	p.stats.Unlock()
	return buffer[:0]
}

// Put returns a buffer to the pool for reuse
// Only buffers with exact capacity matches to predefined sizes will be reused
func (p *pool[T]) Put(buffer []T) {
	if buffer == nil {
		return
	}

	p.stats.Lock()
	p.stats.puts++
	p.stats.Unlock()

	capacity := cap(buffer)

	if pool, exists := p.pools[capacity]; exists {
		buffer = buffer[:0]
		pool.Put(buffer)
	}
}

// findBestSize finds the smallest predefined size that can accommodate the required capacity
// Returns 0 if no suitable size is found
func (p *pool[T]) findBestSize(required int) int {
	for _, size := range p.sizes {
		if size >= required {
			return size
		}
	}
	return 0
}

// Stats returns pool statistics including get/put counts, hits, creates, and hit rate percentage
func (p *pool[T]) Stats() (gets, puts, hits, creates int64, hitRate float64) {
	p.stats.RLock()
	defer p.stats.RUnlock()

	gets = p.stats.gets
	puts = p.stats.puts
	hits = p.stats.hits
	creates = p.stats.creates

	if gets > 0 {
		hitRate = float64(hits) / float64(gets) * 100
	}

	return
}
