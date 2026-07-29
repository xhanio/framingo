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
	if len(p) > remaining {
		p = p[:remaining]
	}
	n, err := lw.w.Write(p)
	lw.written += n
	return n, err
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

// LimitReader reads at most max bytes and then FAILS, rather than reporting a
// clean EOF. That distinction matters for untrusted input: a silently
// truncated body reaches the consumer looking like a complete one. Use it to
// bound anything whose decoded size isn't known from the wire size — an
// inflating stream being the usual case.
type LimitReader interface {
	io.ReadCloser
	Consumed() int
	Remaining() int
}

// NewLimitReader bounds r to max bytes. Reading exactly max succeeds and ends
// in io.EOF; the first byte beyond max returns an error. The error is
// deliberately uncategorized — how an over-long stream maps to a status is the
// caller's decision, not this package's.
//
// It returns a ReadCloser so the result can stand in directly for something
// like http.Request.Body. Close closes r when r is itself a Closer, and is a
// no-op otherwise.
func NewLimitReader(r io.Reader, max int) LimitReader {
	lr := &lreader{
		// max+1 so the overflowing byte is observable rather than swallowed.
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
