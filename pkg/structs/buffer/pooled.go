package buffer

import (
	"io"

	"github.com/xhanio/errors"
)

type pooled[T any] struct {
	data   []T
	pool   PoolG[T]
	pos    int  // read position
	closed bool // whether closed
}

func NewPooledBuffer[T any](size int, pool PoolG[T]) PooledBufferG[T] {
	return newPooledBuffer[T](size, pool)
}

func newPooledBuffer[T any](size int, pool PoolG[T]) *pooled[T] {
	return &pooled[T]{
		data:   pool.Get(size),
		pool:   pool,
		pos:    0,
		closed: false,
	}
}

func (pb *pooled[T]) Write(p []T) (int, error) {
	if pb.closed {
		return 0, errors.Newf("buffer is closed")
	}

	if len(p) == 0 {
		return 0, nil
	}

	requiredCap := len(pb.data) + len(p)

	if requiredCap > cap(pb.data) {
		if err := pb.expand(requiredCap); err != nil {
			return 0, err
		}
	}

	pb.data = append(pb.data, p...)
	return len(p), nil
}

func (pb *pooled[T]) Read(p []T) (int, error) {
	if pb.closed {
		return 0, errors.Newf("buffer is closed")
	}

	if len(p) == 0 {
		return 0, nil
	}

	if pb.pos >= len(pb.data) {
		return 0, io.EOF
	}

	available := len(pb.data) - pb.pos
	toRead := min(len(p), available)

	copy(p[:toRead], pb.data[pb.pos:pb.pos+toRead])
	pb.pos += toRead

	return toRead, nil
}

func (pb *pooled[T]) Close() error {
	if pb.closed {
		return nil // already closed, not an error
	}

	pb.closed = true

	if pb.data != nil {
		pb.pool.Put(pb.data)
		pb.data = nil
	}

	return nil
}

func (pb *pooled[T]) expand(requiredCap int) error {
	if pb.closed {
		return errors.Newf("buffer is closed")
	}

	newCap := requiredCap * 2

	newBuffer := pb.pool.Get(newCap)
	if cap(newBuffer) < requiredCap {
		newBuffer = make([]T, 0, newCap)
	}

	newBuffer = append(newBuffer, pb.data...)

	pb.pool.Put(pb.data)

	pb.data = newBuffer

	return nil
}

// Reset keeps capacity.
func (pb *pooled[T]) Reset() {
	if !pb.closed {
		pb.data = pb.data[:0]
		pb.pos = 0
	}
}

func (pb *pooled[T]) ResetRead() {
	if !pb.closed {
		pb.pos = 0
	}
}

// Seek clamps past the end; a negative position is an error.
func (pb *pooled[T]) Seek(offset int64, whence int) (int64, error) {
	if pb.closed {
		return 0, errors.Newf("buffer is closed")
	}
	var pos int64
	switch whence {
	case io.SeekStart:
		pos = offset
	case io.SeekCurrent:
		pos = int64(pb.pos) + offset
	case io.SeekEnd:
		pos = int64(len(pb.data)) + offset
	default:
		return 0, errors.Newf("invalid whence")
	}
	if pos < 0 {
		return 0, errors.Newf("negative position")
	}
	if pos > int64(len(pb.data)) {
		pos = int64(len(pb.data))
	}
	pb.pos = int(pos)
	return pos, nil
}

func (pb *pooled[T]) Available() int {
	if pb.closed {
		return 0
	}
	return len(pb.data) - pb.pos
}

func (pb *pooled[T]) Len() int     { return len(pb.data) }
func (pb *pooled[T]) Cap() int     { return cap(pb.data) }
func (pb *pooled[T]) Closed() bool { return pb.closed }

// Data returns a copy, nil when closed or empty.
func (pb *pooled[T]) Data() []T {
	if pb.closed || len(pb.data) == 0 {
		return nil
	}
	result := make([]T, len(pb.data))
	copy(result, pb.data)
	return result
}
