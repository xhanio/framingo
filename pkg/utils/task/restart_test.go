package task

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestRestartKeepsTheConcurrencyLimit(t *testing.T) {
	s := newScheduler(MaxConcurrency(1))
	if err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	oldStarted, oldRelease := make(chan struct{}), make(chan struct{})
	old := &Task{Job: holdJob("old", oldStarted, oldRelease), Manual: true}
	if err := s.Add(old); err != nil {
		t.Fatal(err)
	}
	if err := s.Trigger(old.ID(), nil); err != nil {
		t.Fatal(err)
	}
	<-oldStarted
	// the old execution ignores its cancel and goes on past the restart
	if err := s.Stop(true); err != nil {
		t.Fatal(err)
	}
	if err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Stop(true) })

	aStarted, aRelease := make(chan struct{}), make(chan struct{})
	a := &Task{Job: holdJob("a", aStarted, aRelease), Manual: true}
	var bRuns atomic.Int32
	b := &Task{Job: countingJob("b", &bRuns), Manual: true}
	if err := s.Add(a, b); err != nil {
		t.Fatal(err)
	}
	if err := s.Trigger(a.ID(), nil); err != nil {
		t.Fatal(err)
	}
	<-aStarted
	if err := s.Trigger(b.ID(), nil); err != nil {
		t.Fatal(err)
	}
	close(oldRelease)
	time.Sleep(100 * time.Millisecond)
	if n := bRuns.Load(); n != 0 {
		t.Fatal("b ran beside a: the old run's worker freed a slot of the new run")
	}
	close(aRelease)
	eventually(t, "b to run once a is done", func() bool { return bRuns.Load() == 1 })
}

func TestStopLeavesTheQueueEmpty(t *testing.T) {
	// a stop with a task at the handoff could leave the wake-up behind, now and then
	for range 200 {
		s := newScheduler(MaxConcurrency(1))
		if err := s.Start(context.Background()); err != nil {
			t.Fatal(err)
		}
		started, release := make(chan struct{}), make(chan struct{})
		blocker := &Task{Job: holdJob("blocker", started, release), Manual: true}
		var runs atomic.Int32
		held := &Task{Job: countingJob("held", &runs), Manual: true}
		if err := s.Add(blocker, held); err != nil {
			t.Fatal(err)
		}
		if err := s.Trigger(blocker.ID(), nil); err != nil {
			t.Fatal(err)
		}
		<-started
		if err := s.Trigger(held.ID(), nil); err != nil {
			t.Fatal(err)
		}
		eventually(t, "the fetching goroutine to hold a task", s.pq.IsEmpty)
		if err := s.Stop(true); err != nil {
			t.Fatal(err)
		}
		close(release)
		if !s.pq.IsEmpty() {
			t.Fatal("the stop left something in the queue, for the next run to spin on")
		}
	}
}

func TestStartRightAfterAStopWithoutWaiting(t *testing.T) {
	s := newScheduler(MaxConcurrency(1))
	for range 50 {
		if err := s.Start(context.Background()); err != nil {
			t.Fatal(err)
		}
		if err := s.Stop(false); err != nil {
			t.Fatal(err)
		}
		if err := s.Start(context.Background()); err != nil {
			t.Fatal(err)
		}
		var runs atomic.Int32
		if err := s.Add(&Task{Job: countingJob("next", &runs)}); err != nil {
			t.Fatal(err)
		}
		eventually(t, "the next run to run a task", func() bool { return runs.Load() == 1 })
		stopped := make(chan struct{})
		go func() {
			_ = s.Stop(true)
			close(stopped)
		}()
		select {
		case <-stopped:
		case <-time.After(5 * time.Second):
			t.Fatal("Stop(true) hung: a goroutine of an earlier run never woke up")
		}
	}
}

func TestRemoveOfAStaleTaskLeavesItsSuccessorAlone(t *testing.T) {
	t.Run("queued", func(t *testing.T) {
		s := startedScheduler(t, MaxConcurrency(1))
		var staleRuns, runs, heldRuns atomic.Int32
		stale := &Task{Job: countingJob("same", &staleRuns), Manual: true}
		if err := s.Add(stale); err != nil {
			t.Fatal(err)
		}
		s.Remove(stale)
		successor := &Task{Job: countingJob("same", &runs), Manual: true}
		started, release := make(chan struct{}), make(chan struct{})
		blocker := &Task{Job: holdJob("blocker", started, release), Manual: true}
		held := &Task{Job: countingJob("held", &heldRuns), Manual: true}
		if err := s.Add(successor, blocker, held); err != nil {
			t.Fatal(err)
		}
		if err := s.Trigger(blocker.ID(), nil); err != nil {
			t.Fatal(err)
		}
		<-started
		if err := s.Trigger(held.ID(), nil); err != nil {
			t.Fatal(err)
		}
		eventually(t, "the fetching goroutine to hold a task", s.pq.IsEmpty)
		if err := s.Trigger(successor.ID(), nil); err != nil {
			t.Fatal(err)
		}
		s.Remove(stale)
		close(release)
		eventually(t, "the successor to run", func() bool { return runs.Load() == 1 })
	})
	t.Run("scheduled", func(t *testing.T) {
		s := startedScheduler(t)
		var staleRuns, runs atomic.Int32
		stale := &Task{Job: countingJob("same", &staleRuns), Schedule: "0 0 1 1 *"}
		if err := s.Add(stale); err != nil {
			t.Fatal(err)
		}
		s.Remove(stale)
		successor := &Task{Job: countingJob("same", &runs), Schedule: "* * * * * *"}
		if err := s.Add(successor); err != nil {
			t.Fatal(err)
		}
		s.Remove(stale)
		eventually(t, "the successor's schedule to run it", func() bool { return runs.Load() >= 1 })
	})
}
