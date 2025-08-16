package queue

import "io"

var (
	_ io.ReadWriter = (DoubleBufferQueue)(nil)
)

type DoubleBufferQueueG[T any] interface {
	Write(p []T) (int, error)
	Read(p []T) (int, error)
	io.Closer
}

type DoubleBufferQueue = DoubleBufferQueueG[byte]
