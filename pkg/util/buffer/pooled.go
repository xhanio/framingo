package buffer

import (
	"errors"
	"io"
)

type pooled[T any] struct {
	data   []T
	pool   PoolG[T]
	pos    int  // read position
	closed bool // whether closed
}

func NewPooledBuffer[T any](size int) PooledBufferG[T] {
	return newPooledBuffer[T](size)
}

// newPooledBuffer creates a pooled buffer
func newPooledBuffer[T any](size int) *pooled[T] {
	pool := NewPool[T]()
	return &pooled[T]{
		data:   pool.Get(size),
		pool:   pool,
		pos:    0,
		closed: false,
	}
}

// Write implements io.Writer interface
func (pb *pooled[T]) Write(p []T) (int, error) {
	if pb.closed {
		return 0, errors.New("buffer is closed")
	}

	if len(p) == 0 {
		return 0, nil
	}

	requiredCap := len(pb.data) + len(p)

	// check if expansion is needed
	if requiredCap > cap(pb.data) {
		if err := pb.expand(requiredCap); err != nil {
			return 0, err
		}
	}

	pb.data = append(pb.data, p...)
	return len(p), nil
}

// Read implements io.Reader interface
func (pb *pooled[T]) Read(p []T) (int, error) {
	if pb.closed {
		return 0, errors.New("buffer is closed")
	}

	if len(p) == 0 {
		return 0, nil
	}

	// check if there's still data to read
	if pb.pos >= len(pb.data) {
		return 0, io.EOF
	}

	// calculate readable bytes
	available := len(pb.data) - pb.pos
	toRead := min(len(p), available)

	// copy data
	copy(p[:toRead], pb.data[pb.pos:pb.pos+toRead])
	pb.pos += toRead

	return toRead, nil
}

// Close implements io.Closer interface
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

// expand expands the buffer
func (pb *pooled[T]) expand(requiredCap int) error {
	if pb.closed {
		return errors.New("buffer is closed")
	}

	// calculate new capacity (slightly larger than required to reduce frequent expansions)
	newCap := requiredCap * 2

	// get new buffer from pool
	newBuffer := pb.pool.Get(newCap)
	if cap(newBuffer) < requiredCap {
		// pool doesn't have large enough buffer, allocate directly
		newBuffer = make([]T, 0, newCap)
	}

	// copy old data
	newBuffer = append(newBuffer, pb.data...)

	// return old buffer to pool
	pb.pool.Put(pb.data)

	// use new buffer
	pb.data = newBuffer

	return nil
}

// Reset resets the buffer (clears data and read position, but maintains capacity)
func (pb *pooled[T]) Reset() {
	if !pb.closed {
		pb.data = pb.data[:0]
		pb.pos = 0
	}
}

// ResetRead only resets read position, data remains unchanged
func (pb *pooled[T]) ResetRead() {
	if !pb.closed {
		pb.pos = 0
	}
}

// Seek sets read position (similar to file Seek operation)
func (pb *pooled[T]) Seek(offset int64, whence int) (int64, error) {
	if pb.closed {
		return 0, errors.New("buffer is closed")
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
		return 0, errors.New("invalid whence")
	}
	if pos < 0 {
		return 0, errors.New("negative position")
	}
	if pos > int64(len(pb.data)) {
		pos = int64(len(pb.data))
	}
	pb.pos = int(pos)
	return pos, nil
}

// Available returns remaining readable bytes
func (pb *pooled[T]) Available() int {
	if pb.closed {
		return 0
	}
	return len(pb.data) - pb.pos
}

// basic property access
func (pb *pooled[T]) Len() int     { return len(pb.data) }
func (pb *pooled[T]) Cap() int     { return cap(pb.data) }
func (pb *pooled[T]) Closed() bool { return pb.closed }

// Data returns a copy of the buffer (safe access)
func (pb *pooled[T]) Data() []T {
	if pb.closed || len(pb.data) == 0 {
		return nil
	}
	result := make([]T, len(pb.data))
	copy(result, pb.data)
	return result
}
