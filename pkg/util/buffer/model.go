package buffer

import "io"

var (
	_ io.ReadWriter = (PooledBuffer)(nil)
)

type PoolG[T any] interface {
	Get(required int) []T
	Put(buffer []T)
	Stats() (gets, puts, hits, creates int64, hitRate float64)
}

type Pool = PoolG[byte]

type PooledBufferG[T any] interface {
	io.Closer
	io.Seeker
	Write(p []T) (int, error)
	Read(p []T) (int, error)
	Reset()
	ResetRead()
	Available() int
	Len() int
	Cap() int
	Closed() bool
	Data() []T
}

type PooledBuffer = PooledBufferG[byte]
