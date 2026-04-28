package ioutil

import "io"

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
