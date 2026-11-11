package task

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/job"
)

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting: %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func startedManager(t *testing.T, opts ...Option) *manager {
	t.Helper()
	m := newManager(append([]Option{MaxConcurrency(3)}, opts...)...)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Stop(context.Background()) })
	return m
}

func canceled() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

// a schedule that never fires during a test
const neverFires = "0 0 1 1 *"

// a cooldown that outlives every test unless something ends it
const longCooldown = time.Hour

func countingJob(id string, runs *atomic.Int32) job.Job {
	return job.New(id, func(job.Context) error {
		runs.Add(1)
		return nil
	})
}

// failingJob fails its first failures runs and then succeeds, counting every run.
func failingJob(id string, failures int32, runs *atomic.Int32) job.Job {
	return job.New(id, func(job.Context) error {
		if runs.Add(1) <= failures {
			return errors.Newf("run failed")
		}
		return nil
	})
}

func alwaysFailingJob(id string, runs *atomic.Int32) job.Job {
	return failingJob(id, 1<<30, runs)
}

// blockedJob counts its runs and holds each one until release closes or it is canceled.
func blockedJob(id string, runs *atomic.Int32, release <-chan struct{}) job.Job {
	return job.New(id, func(jc job.Context) error {
		runs.Add(1)
		select {
		case <-release:
			return nil
		case <-jc.Context().Done():
			return jc.Context().Err()
		}
	})
}

// holdJob ignores its context: it ends when released, however it was asked to stop.
func holdJob(id string, started chan<- struct{}, release <-chan struct{}) job.Job {
	return job.New(id, func(job.Context) error {
		close(started)
		<-release
		return nil
	})
}

// stubborn ignores its cancel and ends only on the kill signal.
func stubborn(t *testing.T, m *manager) (_ *entity.Task, killed <-chan struct{}) {
	t.Helper()
	started, kill, ended := make(chan struct{}), make(chan struct{}), make(chan struct{})
	t.Cleanup(func() { close(ended) })
	tk := &entity.Task{Job: job.New("stubborn", func(jc job.Context) error {
		close(started)
		select {
		case <-jc.Signaled(job.SignalKill):
			close(kill)
		case <-ended:
		}
		return nil
	}), Manual: true}
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := m.Trigger(tk.ID(), nil); err != nil {
		t.Fatal(err)
	}
	<-started
	return tk, kill
}

// busy reports whether a task is queued, running or being requeued.
func busy(m *manager, tk *entity.Task) bool {
	m.el.RLock()
	defer m.el.RUnlock()
	_, running := m.running[tk]
	reg := m.registered[tk.ID()]
	return running || (reg != nil && (reg.queued || reg.cooldown != nil))
}

func requeueing(m *manager, tk *entity.Task) func() bool {
	return func() bool {
		st := m.Stats(tk.ID())
		return st != nil && st.Requeueing
	}
}

func registrationOf(m *manager, tk *entity.Task) *registration {
	m.el.RLock()
	defer m.el.RUnlock()
	return m.registered[tk.ID()]
}

// forceStop stops with an ended context, which leaves the held execution behind.
func forceStop(t *testing.T, m *manager) {
	t.Helper()
	if err := m.Stop(canceled()); !errors.Is(err, context.Canceled) {
		t.Fatalf("forced stop = %v, want the context's error for what it left", err)
	}
}
