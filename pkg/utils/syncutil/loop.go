package syncutil

import (
	"context"
	"sync"
)

// Loop is the goroutine that takes new work, one per Start. Stop cancels it
// and waits for it to return, with no deadline, so two starts never run it at
// once: run returns as soon as its ctx ends, and hands anything slow to a
// WaitGroup, which Stop waits for only as long as its own ctx allows. Start
// and Stop are the caller's to serialize, as the supervisor does.
type Loop struct {
	mu     sync.Mutex
	cancel context.CancelFunc // nil while stopped
	done   chan struct{}      // closed as the running loop returns
}

// Start runs run in a goroutine, under a context of its own that keeps ctx's
// values, never its cancel or deadline, so Stop alone ends the loop; false,
// running nothing, if a loop runs already.
func (l *Loop) Start(ctx context.Context, run func(ctx context.Context)) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.cancel != nil {
		return false
	}
	loopCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	done := make(chan struct{})
	l.cancel, l.done = cancel, done
	go func() {
		defer close(done)
		run(loopCtx)
	}()
	return true
}

// Stop cancels the loop and returns once it has; false if none was started.
func (l *Loop) Stop() bool {
	l.mu.Lock()
	cancel, done := l.cancel, l.done
	l.cancel, l.done = nil, nil
	l.mu.Unlock()
	if cancel == nil {
		return false
	}
	cancel()
	<-done
	return true
}

// Running is true from Start to Stop, whether or not run has returned on its own.
func (l *Loop) Running() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.cancel != nil
}
