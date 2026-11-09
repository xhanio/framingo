package job

import (
	"context"
	"testing"
	"time"
)

const signalOther Signal = "other"

func received(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

// listening runs a job that reports, once released, whether each signal had arrived and
// whether its context was done.
func listening(t *testing.T, release <-chan struct{}) (*job, <-chan map[string]bool) {
	t.Helper()
	got := make(chan map[string]bool, 1)
	started := make(chan struct{})
	j := newJob("listener", func(jc Context) error {
		close(started)
		<-release
		got <- map[string]bool{
			"kill":     received(jc.Signaled(SignalKill)),
			"other":    received(jc.Signaled(signalOther)),
			"canceled": jc.Context().Err() != nil,
		}
		return nil
	})
	if !j.Run(context.Background(), nil) {
		t.Fatal("the job did not start")
	}
	<-started
	return j, got
}

func TestSignalReachesTheRunningExecution(t *testing.T) {
	release := make(chan struct{})
	j, got := listening(t, release)
	if !j.Signal(SignalKill) {
		t.Fatal("Signal to a running job = false")
	}
	close(release)
	j.Wait()
	g := <-got
	if !g["kill"] {
		t.Fatal("the job did not see the signal")
	}
	if g["other"] {
		t.Fatal("another signal arrived along with it")
	}
	if g["canceled"] {
		t.Fatal("a signal canceled the context")
	}
}

func TestSignalSentBeforeTheJobListensIsKept(t *testing.T) {
	got := make(chan bool, 1)
	started, listen := make(chan struct{}), make(chan struct{})
	j := newJob("late", func(jc Context) error {
		close(started)
		<-listen
		select {
		case <-jc.Signaled(SignalKill):
			got <- true
		case <-time.After(time.Second):
			got <- false
		}
		return nil
	})
	j.Run(context.Background(), nil)
	<-started
	j.Signal(SignalKill)
	close(listen)
	if !<-got {
		t.Fatal("a signal sent before the job listened was lost")
	}
}

func TestSignalWithNothingExecutingIsRefused(t *testing.T) {
	j := newJob("idle", func(Context) error { return nil })
	if j.Signal(SignalKill) {
		t.Fatal("Signal before any run = true")
	}
	j.Run(context.Background(), nil)
	j.Wait()
	if j.Signal(SignalKill) {
		t.Fatal("Signal after the run ended = true")
	}
}

func TestSignalDoesNotCarryOverToTheNextExecution(t *testing.T) {
	release := make(chan struct{})
	j, got := listening(t, release)
	j.Signal(SignalKill)
	close(release)
	j.Wait()
	<-got

	second := make(chan bool, 1)
	j.fn = func(jc Context) error {
		second <- received(jc.Signaled(SignalKill))
		return nil
	}
	j.Run(context.Background(), nil)
	j.Wait()
	if <-second {
		t.Fatal("the last execution's signal reached the next one")
	}
}

func TestSignalReachesAnExecutionBeingCanceled(t *testing.T) {
	release := make(chan struct{})
	j, got := listening(t, release)
	if !j.Cancel() {
		t.Fatal("Cancel of a running job = false")
	}
	if !j.Signal(SignalKill) {
		t.Fatal("Signal to a job being canceled = false")
	}
	close(release)
	j.Wait()
	if g := <-got; !g["kill"] || !g["canceled"] {
		t.Fatalf("the job saw %v", g)
	}
}

func TestSignalSentTwiceIsHarmless(t *testing.T) {
	release := make(chan struct{})
	j, got := listening(t, release)
	if !j.Signal(SignalKill) || !j.Signal(SignalKill) {
		t.Fatal("Signal to a running job = false")
	}
	close(release)
	j.Wait()
	if g := <-got; !g["kill"] {
		t.Fatal("the job did not see the signal")
	}
}
