package syncutil

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// Stop returns only once run has, its stop logic included.
func TestLoopStopWaitsForRunToReturn(t *testing.T) {
	var l Loop
	var returned atomic.Bool
	l.Start(context.Background(), func(ctx context.Context) {
		<-ctx.Done()
		time.Sleep(50 * time.Millisecond) // stop logic
		returned.Store(true)
	})
	if !l.Stop() {
		t.Fatal("Stop = false with a loop running")
	}
	if !returned.Load() {
		t.Fatal("Stop returned before run did")
	}
}

// A Start right after a Stop never runs beside the loop that Stop ended.
func TestLoopStartsNeverOverlap(t *testing.T) {
	var l Loop
	var running, most atomic.Int32
	run := func(ctx context.Context) {
		n := running.Add(1)
		for {
			m := most.Load()
			if n <= m || most.CompareAndSwap(m, n) {
				break
			}
		}
		<-ctx.Done()
		time.Sleep(5 * time.Millisecond)
		running.Add(-1)
	}
	for range 20 {
		l.Start(context.Background(), run)
		l.Stop()
	}
	if most.Load() != 1 {
		t.Fatalf("%d loops ran at once", most.Load())
	}
}

func TestLoopStartWhileRunningRunsNothing(t *testing.T) {
	var l Loop
	var runs atomic.Int32
	run := func(ctx context.Context) {
		runs.Add(1)
		<-ctx.Done()
	}
	if !l.Start(context.Background(), run) {
		t.Fatal("first Start = false")
	}
	if l.Start(context.Background(), run) {
		t.Fatal("second Start = true with a loop running")
	}
	l.Stop()
	if runs.Load() != 1 {
		t.Fatalf("run ran %d times", runs.Load())
	}
}

func TestLoopStopWithoutStart(t *testing.T) {
	var l Loop
	if l.Stop() {
		t.Fatal("Stop = true with nothing started")
	}
	if l.Running() {
		t.Fatal("Running with nothing started")
	}
}

// A run that returns on its own still counts as running until Stop.
func TestLoopRunReturningOnItsOwn(t *testing.T) {
	var l Loop
	returned := make(chan struct{})
	l.Start(context.Background(), func(context.Context) { close(returned) })
	<-returned
	if !l.Running() {
		t.Fatal("Running = false before Stop")
	}
	if !l.Stop() {
		t.Fatal("Stop = false after Start")
	}
	if l.Running() {
		t.Fatal("Running after Stop")
	}
}

type ctxKey struct{}

// Start's ctx bounds the start alone: the loop keeps its values, outlives its
// cancel, and ends with Stop.
func TestLoopOutlivesItsStartContext(t *testing.T) {
	var l Loop
	startCtx, cancel := context.WithCancel(context.WithValue(context.Background(), ctxKey{}, "carried"))
	seen := make(chan any, 1)
	ended := make(chan struct{})
	l.Start(startCtx, func(ctx context.Context) {
		seen <- ctx.Value(ctxKey{})
		<-ctx.Done()
		close(ended)
	})
	if v := <-seen; v != "carried" {
		t.Fatalf("the loop's context carries %v, not Start's values", v)
	}
	cancel()
	select {
	case <-ended:
		t.Fatal("canceling Start's ctx ended the loop")
	case <-time.After(50 * time.Millisecond):
	}
	l.Stop()
	select {
	case <-ended:
	default:
		t.Fatal("Stop returned with the loop still going")
	}
}
