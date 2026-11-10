// Package syncutil holds synchronization helpers the standard library lacks.
package syncutil

import (
	"context"
	"sync"
)

// WaitGroup counts goroutines like sync.WaitGroup, but Wait gives up once its context ends,
// and Add may come at any time, even while a Wait that gave up has left its count going.
type WaitGroup struct {
	mu   sync.Mutex
	n    int
	idle chan struct{} // closed as n drops to 0; nil while it is 0
}

func (g *WaitGroup) Add(delta int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.n == 0 && delta > 0 {
		g.idle = make(chan struct{})
	}
	g.n += delta
	switch {
	case g.n < 0:
		panic("syncutil: negative WaitGroup counter")
	case g.n == 0 && g.idle != nil:
		close(g.idle)
		g.idle = nil
	}
}

func (g *WaitGroup) Done() {
	g.Add(-1)
}

// Go runs f in a goroutine the group counts.
func (g *WaitGroup) Go(f func()) {
	g.Add(1)
	go func() {
		defer g.Done()
		f()
	}()
}

// Wait returns nil once the count drops to zero, or ctx's error if ctx ends first.
func (g *WaitGroup) Wait(ctx context.Context) error {
	g.mu.Lock()
	idle := g.idle
	g.mu.Unlock()
	if idle == nil {
		return nil
	}
	select {
	case <-idle:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
