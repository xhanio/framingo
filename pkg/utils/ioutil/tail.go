package ioutil

import (
	"io"
	"sync"
)

type TailWriter interface {
	io.Writer
	Bytes() []byte
	String() string
}

func NewTailWriter(limit int) TailWriter {
	return &twriter{ring: make([]byte, max(limit, 0))}
}

type twriter struct {
	mu      sync.Mutex
	ring    []byte
	next    int
	wrapped bool
}

func (tw *twriter) Write(p []byte) (int, error) {
	size := len(tw.ring)
	if size == 0 {
		return len(p), nil
	}
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if len(p) >= size {
		copy(tw.ring, p[len(p)-size:])
		tw.next, tw.wrapped = 0, true
		return len(p), nil
	}
	if n := copy(tw.ring[tw.next:], p); n < len(p) {
		copy(tw.ring, p[n:])
	}
	tw.next += len(p)
	if tw.next >= size {
		tw.next -= size
		tw.wrapped = true
	}
	return len(p), nil
}

func (tw *twriter) Bytes() []byte {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if !tw.wrapped {
		return append([]byte(nil), tw.ring[:tw.next]...)
	}
	out := make([]byte, 0, len(tw.ring))
	out = append(out, tw.ring[tw.next:]...)
	return append(out, tw.ring[:tw.next]...)
}

func (tw *twriter) String() string {
	return string(tw.Bytes())
}
