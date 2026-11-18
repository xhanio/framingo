package ioutil

import (
	"io"

	"github.com/xhanio/errors"
)

type LimitWriter interface {
	io.Writer
	Written() int
	Remaining() int
}

func NewLimitWriter(w io.Writer, max int) LimitWriter {
	return &lwriter{w: w, max: max}
}

type lwriter struct {
	w       io.Writer
	max     int
	written int
}

func (lw *lwriter) Write(p []byte) (int, error) {
	remaining := lw.max - lw.written
	if remaining <= 0 {
		return len(p), nil
	}
	whole := len(p)
	if whole > remaining {
		p = p[:remaining]
	}
	n, err := lw.w.Write(p)
	lw.written += n
	if err != nil {
		return n, err
	}
	// excess is discarded but reported written: a short count is an error to callers
	return whole, nil
}

func (lw *lwriter) Written() int {
	return lw.written
}

func (lw *lwriter) Remaining() int {
	r := lw.max - lw.written
	if r < 0 {
		return 0
	}
	return r
}

// LimitReader errors past max rather than returning EOF, so truncated untrusted input never looks complete.
type LimitReader interface {
	io.ReadCloser
	Consumed() int
	Remaining() int
}

// NewLimitReader: exactly max bytes ends in io.EOF; the error is uncategorized so the caller picks
// the status. Close closes r if it is a Closer.
func NewLimitReader(r io.Reader, max int) LimitReader {
	lr := &lreader{
		// max+1 so the overflowing byte is observable
		r:   io.LimitReader(r, int64(max)+1),
		max: max,
	}
	if c, ok := r.(io.Closer); ok {
		lr.c = c
	}
	return lr
}

type lreader struct {
	r        io.Reader
	c        io.Closer
	max      int
	consumed int
}

func (lr *lreader) Close() error {
	if lr.c == nil {
		return nil
	}
	return lr.c.Close()
}

func (lr *lreader) Read(p []byte) (int, error) {
	if lr.consumed > lr.max {
		return 0, errors.Newf("read limit of %d bytes exceeded", lr.max)
	}
	n, err := lr.r.Read(p)
	lr.consumed += n
	if lr.consumed > lr.max {
		return 0, errors.Newf("read limit of %d bytes exceeded", lr.max)
	}
	return n, err
}

func (lr *lreader) Consumed() int {
	return lr.consumed
}

func (lr *lreader) Remaining() int {
	r := lr.max - lr.consumed
	if r < 0 {
		return 0
	}
	return r
}
