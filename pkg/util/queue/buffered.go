package queue

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"xhanio/framingo/pkg/util/buffer"
)

// buffered[T] double buffer buffered using object pool
type buffered[T any] struct {
	writeBuffer buffer.PooledBufferG[T]
	readBuffer  buffer.PooledBufferG[T]

	// synchronization control
	sync.RWMutex
	written *sync.Cond

	// lifecycle management
	wg     *sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc

	// configuration
	swapInterval time.Duration
	swapTicker   *time.Ticker
}

func NewDoubleBufferQueue[T any](ctx context.Context, initialSize int, swapInterval time.Duration) DoubleBufferQueueG[T] {
	return newDoubleBufferQueue[T](ctx, initialSize, swapInterval)
}

// newDoubleBufferQueue creates a pooled double buffer buffered
func newDoubleBufferQueue[T any](ctx context.Context, initialSize int, swapInterval time.Duration) *buffered[T] {
	if ctx == nil {
		ctx = context.Background()
	}
	q := &buffered[T]{
		writeBuffer:  buffer.NewPooledBuffer[T](initialSize),
		readBuffer:   buffer.NewPooledBuffer[T](initialSize),
		wg:           &sync.WaitGroup{},
		swapInterval: swapInterval,
		swapTicker:   time.NewTicker(swapInterval),
	}
	q.ctx, q.cancel = context.WithCancel(ctx)
	q.written = sync.NewCond(q)

	// start background processing goroutine
	q.wg.Add(1)
	go q.processLoop()
	return q
}

// Write implements io.Writer interface
func (q *buffered[T]) Write(p []T) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	q.Lock()
	defer q.Unlock()

	if q.cancel == nil {
		return 0, errors.New("buffered is closed")
	}

	// write data to write buffer
	n, err := q.writeBuffer.Write(p)
	if err != nil {
		return 0, err
	}

	// immediately try to swap for better responsiveness
	if q.readBuffer.Available() == 0 && q.writeBuffer.Len() > 0 {
		q.doSwap()
	}

	return n, nil
}

// Read implements io.Reader interface
func (q *buffered[T]) Read(p []T) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	q.Lock()
	defer q.Unlock()

	// try to swap buffers if read buffer is empty
	if q.readBuffer.Available() == 0 && q.writeBuffer.Len() > 0 {
		q.doSwap()
	}

	// check if there's data available to read
	if q.readBuffer.Available() == 0 {
		if q.cancel == nil {
			return 0, io.EOF
		}
		return 0, nil // no data available, non-blocking
	}

	// read data
	n, err := q.readBuffer.Read(p)

	// if read buffer is empty, reset it
	if q.readBuffer.Available() == 0 {
		q.readBuffer.Reset()
		// read buffer reset, no need to signal anything
	}

	return n, err
}

// swapBuffers swaps read/write buffers (concurrency safe)
func (q *buffered[T]) swapBuffers() {
	q.Lock()
	defer q.Unlock()
	q.doSwap()
}

// doSwap internal swap logic (must be called while holding lock)
func (q *buffered[T]) doSwap() {
	// only swap when write buffer has data and read buffer is empty
	if q.writeBuffer.Len() > 0 && q.readBuffer.Available() == 0 {
		// swap buffers
		q.writeBuffer, q.readBuffer = q.readBuffer, q.writeBuffer

		// reset new write buffer
		q.writeBuffer.Reset()

		// notify waiting read operations
		q.written.Broadcast()
	}
}

// processLoop background processing loop
func (q *buffered[T]) processLoop() {
	defer q.wg.Done()
	if q.swapTicker != nil {
		defer q.swapTicker.Stop()
	}
	for {
		select {
		case <-q.ctx.Done():
			q.swapBuffers() // final swap
			return
		case <-q.swapTicker.C:
			q.swapBuffers()
		}
	}
}

// Close closes the buffered
func (q *buffered[T]) Close() error {
	q.Lock()
	if q.cancel == nil {
		q.Unlock()
		return nil
	}
	q.cancel()
	q.cancel = nil
	q.Unlock()

	// stop background processing
	q.wg.Wait()

	// notify all waiting operations (must be done with lock held)
	q.Lock()
	q.written.Broadcast()
	q.Unlock()

	// return buffers to pool
	q.writeBuffer.Close()
	q.readBuffer.Close()
	return nil
}
