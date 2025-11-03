package ioutil

import (
	"context"
	"io"
	"sync"

	"github.com/xhanio/errors"
)

type ProgressWriter interface {
	io.Writer
	Written() int64
	Percentage() float64
}

type pwriter struct {
	sync.RWMutex
	ctx     context.Context
	w       io.Writer
	current int64
	total   int64
}

func NewProgressWriter(ctx context.Context, w io.Writer, size int64) ProgressWriter {
	return &pwriter{
		ctx:     ctx,
		w:       w,
		current: 0,
		total:   size,
	}
}

func (pw *pwriter) Write(p []byte) (int, error) {
	select {
	case <-pw.ctx.Done():
		return 0, errors.Cancaled.Wrap(pw.ctx.Err()) // Upload canceled
	default:
		n, err := pw.w.Write(p)
		pw.Lock()
		defer pw.Unlock()
		pw.current += int64(n)
		return n, err
	}
}

func (pw *pwriter) Written() int64 {
	pw.RLock()
	defer pw.RUnlock()
	return pw.current
}

func (pw *pwriter) Percentage() float64 {
	pw.RLock()
	defer pw.RUnlock()
	if pw.total == 0 {
		return 0
	}
	return float64(pw.current) / float64(pw.total) * 100
}
