package syncutil

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWaitWithNothingCountedReturnsAtOnce(t *testing.T) {
	var g WaitGroup
	if err := g.Wait(context.Background()); err != nil {
		t.Fatalf("Wait = %v", err)
	}
}

func TestWaitReturnsOnceTheGoroutinesEnd(t *testing.T) {
	var g WaitGroup
	release := make(chan struct{})
	for range 3 {
		g.Go(func() { <-release })
	}
	waited := make(chan error, 1)
	go func() { waited <- g.Wait(context.Background()) }()
	select {
	case err := <-waited:
		t.Fatalf("Wait = %v with goroutines still going", err)
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	select {
	case err := <-waited:
		if err != nil {
			t.Fatalf("Wait = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Wait did not return once the goroutines ended")
	}
}

func TestWaitGivesUpOnceTheContextEnds(t *testing.T) {
	var g WaitGroup
	release := make(chan struct{})
	defer close(release)
	g.Go(func() { <-release })
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := g.Wait(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Wait = %v, want the context's error", err)
	}
}

func TestCountingAgainWhileAWaitGaveUp(t *testing.T) {
	var g WaitGroup
	first := make(chan struct{})
	g.Go(func() { <-first })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := g.Wait(ctx); err == nil {
		t.Fatal("Wait = nil with a goroutine still going")
	}
	second := make(chan struct{})
	g.Go(func() { <-second })
	close(first)
	waited := make(chan error, 1)
	go func() { waited <- g.Wait(context.Background()) }()
	select {
	case err := <-waited:
		t.Fatalf("Wait = %v with the second goroutine still going", err)
	case <-time.After(50 * time.Millisecond):
	}
	close(second)
	if err := <-waited; err != nil {
		t.Fatalf("Wait = %v", err)
	}
}

func TestDoneBelowZeroPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Done on an empty group did not panic")
		}
	}()
	var g WaitGroup
	g.Done()
}
