package executor

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/utils/job"
)

func blockingJob(runs *atomic.Int32, started chan<- struct{}) job.Job {
	return job.New("", job.Wrap(func(ctx context.Context) error {
		runs.Add(1)
		started <- struct{}{}
		<-ctx.Done()
		return errors.Newf("stopped: %v", ctx.Err())
	}))
}

func waitStart(t *testing.T, done <-chan error, within time.Duration) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(within):
		t.Fatalf("Start did not return within %s", within)
		return nil
	}
}

func TestRetryStopsWhenJobCanceled(t *testing.T) {
	var runs atomic.Int32
	started := make(chan struct{}, 10)
	j := blockingJob(&runs, started)
	done := make(chan error, 1)
	go func() { done <- New(j, WithRetry(3, 10*time.Millisecond)).Start(context.Background(), nil) }()
	<-started
	j.Cancel()
	if err := waitStart(t, done, 2*time.Second); err == nil {
		t.Fatal("a canceled job must not report success")
	}
	if n := runs.Load(); n != 1 {
		t.Fatalf("canceled job ran %d times, want 1", n)
	}
}

func TestRetryStopsWhenContextCanceled(t *testing.T) {
	var runs atomic.Int32
	started := make(chan struct{}, 10)
	j := blockingJob(&runs, started)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- New(j, WithRetry(5, 0)).Start(ctx, nil) }()
	<-started
	cancel()
	if err := waitStart(t, done, 2*time.Second); err == nil {
		t.Fatal("a job stopped by its caller's context must not report success")
	}
	if n := runs.Load(); n != 1 {
		t.Fatalf("job ran %d times after its caller's context ended, want 1", n)
	}
}

func TestRetryBackoffEndsWhenContextCanceled(t *testing.T) {
	j := job.New("", job.Wrap(func(context.Context) error { return errors.Newf("boom") }))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- New(j, WithRetry(3, time.Hour)).Start(ctx, nil) }()
	time.Sleep(50 * time.Millisecond)
	cancel()
	if err := waitStart(t, done, 2*time.Second); err == nil {
		t.Fatal("a failing job must not report success")
	}
}

func TestRetryDoesNotQueueBehindRunningJob(t *testing.T) {
	var runs atomic.Int32
	started := make(chan struct{}, 10)
	release := make(chan struct{})
	j := job.New("", job.Wrap(func(context.Context) error {
		runs.Add(1)
		started <- struct{}{}
		<-release
		return nil
	}))
	go func() { _ = New(j).Start(context.Background(), nil) }()
	<-started
	// ends the first run while the second executor would still be retrying
	go func() {
		time.Sleep(30 * time.Millisecond)
		close(release)
	}()
	if err := New(j, WithRetry(5, 20*time.Millisecond)).Start(context.Background(), nil); err == nil {
		t.Fatal("starting a job that is still running must fail")
	}
	time.Sleep(100 * time.Millisecond)
	if n := runs.Load(); n != 1 {
		t.Fatalf("job ran %d times, want 1: a start refused because the job was busy must not run it later", n)
	}
}

func TestRetryCountsRetriesNotRuns(t *testing.T) {
	tests := []struct {
		retries  int
		wantRuns int32
	}{
		{0, 1},
		{1, 2},
		{3, 4},
	}
	for _, tt := range tests {
		var runs atomic.Int32
		j := job.New("", job.Wrap(func(context.Context) error {
			runs.Add(1)
			return errors.Newf("boom")
		}))
		if err := New(j, WithRetry(tt.retries, time.Millisecond)).Start(context.Background(), nil); err == nil {
			t.Fatalf("WithRetry(%d): a job that always fails must not report success", tt.retries)
		}
		if n := runs.Load(); n != tt.wantRuns {
			t.Errorf("WithRetry(%d): job ran %d times, want %d (the first run plus %d retries)", tt.retries, n, tt.wantRuns, tt.retries)
		}
	}
}
