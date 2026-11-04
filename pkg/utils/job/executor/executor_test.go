package executor

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/utils/job"
)

func TestJobRetry(t *testing.T) {
	j := job.New("", job.Wrap(func(ctx context.Context) error {
		for {
			select {
			case <-ctx.Done():
				if ctx.Err() == context.DeadlineExceeded {
					return errors.Newf("job timed out")
				} else {
					return errors.Newf("job canceled")
				}
			case <-time.After(2 * time.Second):
				return errors.Newf("error occurred")
			}
		}
	}))

	t.Logf("job id is %s", j.ID())
	je := New(j, WithRetry(2, 1*time.Second))
	err := je.Start(context.Background(), nil)
	if err == nil {
		t.Fatal("failed job completed without error")
	}
	if !strings.Contains(err.Error(), "error occurred") {
		t.Fatalf("failed job completed with incorrect error: %v", j.Err())
	}
}

func TestJobTimedOut(t *testing.T) {
	j := job.New("", job.Wrap(func(ctx context.Context) error {
		for {
			select {
			case <-ctx.Done():
				if ctx.Err() == context.DeadlineExceeded {
					return errors.Newf("job timed out")
				} else {
					return errors.Newf("job canceled")
				}
			case <-time.After(2 * time.Second):
				return nil
			}
		}
	}))
	t.Logf("job id is %s", j.ID())
	je := New(j, WithTimeout(1*time.Second))
	err := je.Start(context.Background(), nil)
	if err == nil || err.Error() != "job timed out" {
		t.Fatal("job completed without error or with incorrect err message")
	}
}

func TestJobCanceled(t *testing.T) {
	j := job.New("", job.Wrap(func(ctx context.Context) error {
		for {
			select {
			case <-ctx.Done():
				if ctx.Err() == context.DeadlineExceeded {
					return errors.Newf("job timed out")
				} else {
					return errors.Newf("job canceled")
				}
			case <-time.After(4 * time.Second):
				return nil
			}
		}
	}))
	t.Logf("job id is %s", j.ID())
	je := New(j, WithTimeout(3*time.Second))
	go func() {
		<-time.After(2 * time.Second)
		j.Cancel()
	}()
	err := je.Start(context.Background(), nil)
	if err == nil || !j.IsState(job.StateCanceled) {
		t.Fatalf("job completed without error or with incorrect job state %s and error %s", j.State(), j.Err().Error())
	}
}

func TestJobSuccess(t *testing.T) {
	j := job.New("", job.Wrap(func(ctx context.Context) error {
		return nil
	}))

	je := New(j)
	err := je.Start(context.Background(), nil)
	if err != nil {
		t.Fatalf("successful job failed with error: %v", err)
	}
}

func TestJobSuccessWithTimeout(t *testing.T) {
	j := job.New("", job.Wrap(func(ctx context.Context) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	}))

	je := New(j, WithTimeout(500*time.Millisecond))
	err := je.Start(context.Background(), nil)
	if err != nil {
		t.Fatalf("successful job failed with error: %v", err)
	}
}

func TestStop(t *testing.T) {
	j := job.New("", job.Wrap(func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return errors.Newf("job canceled")
		case <-time.After(5 * time.Second):
			return nil
		}
	}))

	je := New(j)
	go func() {
		time.Sleep(100 * time.Millisecond)
		err := je.Stop(true)
		if err != nil {
			t.Errorf("Stop failed with error: %v", err)
		}
	}()

	err := je.Start(context.Background(), nil)
	if err == nil || err.Error() != "job canceled" {
		t.Fatalf("expected 'job canceled' error, got: %v", err)
	}
}

func TestStopWithoutWait(t *testing.T) {
	j := job.New("", job.Wrap(func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return errors.Newf("job canceled")
		case <-time.After(5 * time.Second):
			return nil
		}
	}))

	je := New(j)
	go func() {
		time.Sleep(100 * time.Millisecond)
		err := je.Stop(false)
		if err != nil {
			t.Errorf("Stop failed with error: %v", err)
		}
	}()

	err := je.Start(context.Background(), nil)
	if err == nil {
		t.Fatal("job should have been canceled")
	}
}

func TestStats(t *testing.T) {
	j := job.New("", job.Wrap(func(ctx context.Context) error {
		return errors.Newf("error occurred")
	}))

	je := New(j, WithRetry(3, 100*time.Millisecond))
	_ = je.Start(context.Background(), nil)

	stats := je.Stats()
	if stats == nil {
		t.Fatal("Stats should not be nil")
	}
	if stats.Retries != 3 {
		t.Errorf("expected 3 retries, got %d", stats.Retries)
	}
}

func TestStatsWithoutRetry(t *testing.T) {
	j := job.New("", job.Wrap(func(ctx context.Context) error {
		return nil
	}))

	je := New(j)
	err := je.Start(context.Background(), nil)
	if err != nil {
		t.Fatalf("job failed: %v", err)
	}

	stats := je.Stats()
	if stats == nil {
		t.Fatal("Stats should not be nil")
	}
	if stats.Retries != 0 {
		t.Errorf("expected 0 retries, got %d", stats.Retries)
	}
}

func TestReset(t *testing.T) {
	j := job.New("", job.Wrap(func(ctx context.Context) error {
		return errors.Newf("error occurred")
	}))

	je := New(j, WithRetry(2, 100*time.Millisecond))

	// First execution with retries
	_ = je.Start(context.Background(), nil)

	stats := je.Stats()
	if stats.Retries != 2 {
		t.Errorf("expected 2 retries, got %d", stats.Retries)
	}

	// Reset the executor
	je.(*executor).Reset()

	// Stats should show reset values
	stats = je.Stats()
	if stats.Retries != 0 {
		t.Errorf("expected 0 retries after reset, got %d", stats.Retries)
	}
}

func TestNilContext(t *testing.T) {
	j := job.New("", job.Wrap(func(ctx context.Context) error {
		return nil
	}))

	je := New(j)
	// Test that nil context is handled (converted to Background)
	err := je.Start(nil, nil) //nolint:staticcheck // Testing nil context handling
	if err != nil {
		t.Fatalf("job with nil context failed: %v", err)
	}
}

func TestInvalidRetryAttempts(t *testing.T) {
	j := job.New("", job.Wrap(func(ctx context.Context) error {
		return errors.Newf("error")
	}))

	// WithRetry with 0 or negative attempts should be ignored
	je := New(j, WithRetry(0, 1*time.Second))
	err := je.Start(context.Background(), nil)
	// Should fail immediately without retries
	if err == nil {
		t.Fatal("expected error without retries")
	}

	stats := je.Stats()
	if stats.Retries != 0 {
		t.Errorf("expected 0 retries with invalid attempts, got %d", stats.Retries)
	}
}

func TestInvalidTimeout(t *testing.T) {
	j := job.New("", job.Wrap(func(ctx context.Context) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	}))

	// WithTimeout with 0 or negative duration should be ignored
	je := New(j, WithTimeout(0))
	err := je.Start(context.Background(), nil)
	if err != nil {
		t.Fatalf("job failed: %v", err)
	}
}

func TestNoTimeout(t *testing.T) {
	j := job.New("", job.Wrap(func(ctx context.Context) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	}))

	je := New(j, NoTimeout())
	err := je.Start(context.Background(), nil)
	if err != nil {
		t.Fatalf("job with NoTimeout failed: %v", err)
	}
}

func TestJobAlreadyRunning(t *testing.T) {
	j := job.New("", job.Wrap(func(ctx context.Context) error {
		time.Sleep(500 * time.Millisecond)
		return nil
	}))

	je := New(j)

	// Start the job
	go func() {
		_ = je.Start(context.Background(), nil)
	}()

	time.Sleep(50 * time.Millisecond) // Let it start

	// Try to start again while running
	err := je.Start(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "still pending") {
		t.Fatalf("expected 'still pending' error, got: %v", err)
	}
}

func TestRetryWithSuccess(t *testing.T) {
	attempt := 0
	j := job.New("", job.Wrap(func(ctx context.Context) error {
		attempt++
		if attempt < 2 {
			return errors.Newf("temporary error")
		}
		return nil
	}))

	je := New(j, WithRetry(3, 100*time.Millisecond))
	err := je.Start(context.Background(), nil)
	if err != nil {
		t.Fatalf("job should succeed after retry, got: %v", err)
	}

	stats := je.Stats()
	if stats.Retries != 1 {
		t.Errorf("expected 1 retry (failed once, succeeded on second), got %d", stats.Retries)
	}
}

func TestStartRunsADoneJobAgain(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{name: "succeeded", err: nil},
		{name: "failed", err: errors.Newf("error")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runs := 0
			je := New(job.New("", job.Wrap(func(ctx context.Context) error {
				runs++
				return tc.err
			})))
			_ = je.Start(context.Background(), nil)
			_ = je.Start(context.Background(), nil)
			if runs != 2 {
				t.Fatalf("runs = %d, want 2", runs)
			}
		})
	}
}
