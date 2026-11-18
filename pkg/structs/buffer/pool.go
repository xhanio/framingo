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

// NewPool defaults to sizes 1K, 4K, 16K, 64K, 256K and 1M elements.
func NewPool[T any](sizes ...int) PoolG[T] {
	return newPool[T](sizes...)
}

func newPool[T any](sizes ...int) *pool[T] {
	if len(sizes) == 0 {
		sizes = []int{1024, 4096, 16384, 65536, 262144, 1048576}
	}

	p := &pool[T]{
		pools: make(map[int]*sync.Pool),
		sizes: sizes,
	}

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

// Get returns an empty slice with capacity >= required, allocating when no size class fits.
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

// Put keeps only buffers whose capacity exactly matches a size class.
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
		//nolint:staticcheck // SA6002: the header allocation per Put is accepted over switching to *[]T
		pool.Put(buffer)
	}
}

// findBestSize returns 0 when no size class fits.
func (p *pool[T]) findBestSize(required int) int {
	for _, size := range p.sizes {
		if size >= required {
			return size
		}
	}
	return 0
}

// Stats reports hitRate as a percentage.
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
