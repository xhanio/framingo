package queue

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/structs/buffer"
)

type buffered[T any] struct {
	writeBuffer buffer.PooledBufferG[T]
	readBuffer  buffer.PooledBufferG[T]

	sync.RWMutex
	written *sync.Cond

	wg     *sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc

	swapInterval time.Duration
	swapTicker   *time.Ticker
}

func NewDoubleBufferQueue[T any](ctx context.Context, initialSize int, swapInterval time.Duration) DoubleBufferQueueG[T] {
	return newDoubleBufferQueue[T](ctx, initialSize, swapInterval)
}

func newDoubleBufferQueue[T any](ctx context.Context, initialSize int, swapInterval time.Duration) *buffered[T] {
	if ctx == nil {
		ctx = context.Background()
	}
	q := &buffered[T]{
		writeBuffer:  buffer.NewPooledBuffer(initialSize, buffer.NewPool[T]()),
		readBuffer:   buffer.NewPooledBuffer(initialSize, buffer.NewPool[T]()),
		wg:           &sync.WaitGroup{},
		swapInterval: swapInterval,
		swapTicker:   time.NewTicker(swapInterval),
	}
	q.ctx, q.cancel = context.WithCancel(ctx)
	q.written = sync.NewCond(q)

	q.wg.Add(1)
	go q.processLoop()
	return q
}

func (q *buffered[T]) Write(p []T) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	q.Lock()
	defer q.Unlock()

	if q.cancel == nil {
		return 0, errors.Newf("buffered is closed")
	}

	n, err := q.writeBuffer.Write(p)
	if err != nil {
		return 0, err
	}

	if q.readBuffer.Available() == 0 && q.writeBuffer.Len() > 0 {
		q.doSwap()
	}

	return n, nil
}

// Read never blocks: (0, nil) when empty, io.EOF once closed and drained.
func (q *buffered[T]) Read(p []T) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	q.Lock()
	defer q.Unlock()

	if q.readBuffer.Available() == 0 && q.writeBuffer.Len() > 0 {
		q.doSwap()
	}

	if q.readBuffer.Available() == 0 {
		if q.cancel == nil {
			return 0, io.EOF
		}
		return 0, nil // no data available, non-blocking
	}

	n, err := q.readBuffer.Read(p)

	if q.readBuffer.Available() == 0 {
		q.readBuffer.Reset()
	}

	return n, err
}

func (q *buffered[T]) swapBuffers() {
	q.Lock()
	defer q.Unlock()
	q.doSwap()
}

// doSwap requires the lock held.
func (q *buffered[T]) doSwap() {
	if q.writeBuffer.Len() > 0 && q.readBuffer.Available() == 0 {
		q.writeBuffer, q.readBuffer = q.readBuffer, q.writeBuffer

		q.writeBuffer.Reset()

		q.written.Broadcast()
	}
}

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

func (q *buffered[T]) Close() error {
	q.Lock()
	if q.cancel == nil {
		q.Unlock()
		return nil
	}
	q.cancel()
	q.cancel = nil
	q.Unlock()

	q.wg.Wait()

	// Broadcast must hold the lock
	q.Lock()
	q.written.Broadcast()
	q.Unlock()

	q.writeBuffer.Close()
	q.readBuffer.Close()
	return nil
}
