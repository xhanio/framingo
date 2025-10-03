package job

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/xhanio/framingo/pkg/utils/log"
)

func TestJobCompleted(t *testing.T) {
	j := New("", Wrap(func(ctx context.Context) error {
		for {
			select {
			case <-ctx.Done():
				if ctx.Err() == context.DeadlineExceeded {
					return errors.New("job timed out")
				} else {
					return errors.New("job canceled")
				}
			case <-time.After(2 * time.Second):
				return nil
			}
		}
	}))
	t.Logf("job id is %s", j.ID())
	j.Run(context.Background(), nil)
	j.Wait()
	if j.Err() != nil {
		t.Fatalf("completed job failed with error: %s", j.Err().Error())
	}
}

func TestJobPanicked(t *testing.T) {
	j := New("", Wrap(func(ctx context.Context) error {
		for {
			select {
			case <-ctx.Done():
				if ctx.Err() == context.DeadlineExceeded {
					return errors.New("job timed out")
				} else {
					return errors.New("job canceled")
				}
			case <-time.After(2 * time.Second):
				panic(errors.New("?!"))
			}
		}
	}))
	t.Logf("job id is %s", j.ID())
	j.Run(context.Background(), nil)
	j.Wait()
	if j.Err() == nil || j.Err().Error() != "?!" {
		t.Fatal("panicked job completed without error or with incorrect err message")
	}
}

func TestJobCanceled(t *testing.T) {
	j := New("", Wrap(func(ctx context.Context) error {
		for {
			select {
			case <-ctx.Done():
				if ctx.Err() == context.DeadlineExceeded {
					return errors.New("job timed out")
				} else {
					return errors.New("job canceled")
				}
			case <-time.After(4 * time.Second):
				return nil
			}
		}
	}))
	t.Logf("job id is %s", j.ID())
	go func() {
		<-time.After(2 * time.Second)
		j.Cancel()
	}()
	j.Run(context.Background(), nil)
	j.Wait()
	err := j.Err()
	if err == nil || !j.IsState(StateCanceled) {
		t.Fatalf("job completed without error or with incorrect job state %s and error %s", j.State(), j.Err().Error())
	}
}

func TestJobFailed(t *testing.T) {
	j := New("", Wrap(func(ctx context.Context) error {
		return errors.New("job failed")
	}))
	j.Run(context.Background(), nil)
	j.Wait()
	if j.Err() == nil || !j.IsState(StateFailed) {
		t.Fatal("job should have failed with error")
	}
}

func TestJobGetters(t *testing.T) {
	testID := "test-job-123"
	testParams := map[string]string{"key": "value"}

	j := New(testID, Wrap(func(ctx context.Context) error {
		return nil
	}))

	// Test ID
	if j.ID() != testID {
		t.Errorf("expected ID %s, got %s", testID, j.ID())
	}

	// Test CreatedAt
	if j.CreatedAt().IsZero() {
		t.Error("CreatedAt should not be zero")
	}

	// Test Context before run
	if j.Context() == nil {
		t.Error("Context should not be nil even before run")
	}

	j.Run(context.Background(), testParams)
	j.Wait()

	// Test StartedAt and EndedAt
	if j.StartedAt().IsZero() {
		t.Error("StartedAt should not be zero after run")
	}
	if j.EndedAt().IsZero() {
		t.Error("EndedAt should not be zero after completion")
	}

	// Test ExecutionTime
	execTime := j.ExecutionTime()
	if execTime < 0 {
		t.Error("ExecutionTime should be positive")
	}
}

func TestJobProgress(t *testing.T) {
	j := New("", func(ctx Context) error {
		ctx.SetProgress(0.5)
		time.Sleep(100 * time.Millisecond)
		ctx.SetProgress(1.0)
		return nil
	})

	j.Run(context.Background(), nil)
	j.Wait()

	progress := j.Progress()
	if progress != 1.0 {
		t.Errorf("expected progress 1.0, got %f", progress)
	}
}

func TestJobResultAndParams(t *testing.T) {
	testParams := map[string]string{"key": "value"}
	testResult := "test result"

	j := New("", func(ctx Context) error {
		params := ctx.GetParams()
		if params == nil {
			return errors.New("params should not be nil")
		}
		ctx.SetResult(testResult)
		return nil
	})

	j.Run(context.Background(), testParams)
	j.Wait()

	result := j.Result()
	if result != testResult {
		t.Errorf("expected result %v, got %v", testResult, result)
	}
}

func TestJobStats(t *testing.T) {
	testID := "stats-job"
	j := New(testID, Wrap(func(ctx context.Context) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	}))

	j.Run(context.Background(), nil)
	j.Wait()

	stats := j.Stats()
	if stats == nil {
		t.Fatal("Stats should not be nil")
	}
	if stats.ID != testID {
		t.Errorf("expected ID %s, got %s", testID, stats.ID)
	}
	if stats.State != string(StateSucceeded) {
		t.Errorf("expected state %s, got %s", StateSucceeded, stats.State)
	}
	if stats.ExecutionTime < 100*time.Millisecond {
		t.Error("execution time should be at least 100ms")
	}
}

func TestJobStatsWithError(t *testing.T) {
	expectedErr := "stats error"
	j := New("", Wrap(func(ctx context.Context) error {
		return errors.New(expectedErr)
	}))

	j.Run(context.Background(), nil)
	j.Wait()

	stats := j.Stats()
	if stats.Error != expectedErr {
		t.Errorf("expected error %s, got %s", expectedErr, stats.Error)
	}
}

func TestJobIsDone(t *testing.T) {
	j := New("", Wrap(func(ctx context.Context) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	}))

	if j.IsDone() {
		t.Error("job should not be done before running")
	}

	j.Run(context.Background(), nil)
	time.Sleep(10 * time.Millisecond) // Give it time to start

	if j.IsDone() {
		t.Error("job should not be done while running")
	}

	j.Wait()

	if !j.IsDone() {
		t.Error("job should be done after completion")
	}
}

func TestJobIsExecuting(t *testing.T) {
	j := New("", Wrap(func(ctx context.Context) error {
		time.Sleep(200 * time.Millisecond)
		return nil
	}))

	if j.IsExecuting() {
		t.Error("job should not be executing before run")
	}

	j.Run(context.Background(), nil)
	time.Sleep(50 * time.Millisecond)

	if !j.IsExecuting() {
		t.Error("job should be executing during run")
	}

	j.Wait()

	if j.IsExecuting() {
		t.Error("job should not be executing after completion")
	}
}

func TestJobOptions(t *testing.T) {
	testKey := "env"
	testVal := "test"

	j1 := New("",
		Wrap(func(ctx context.Context) error { return nil }),
		WithLabel(testKey, testVal),
	)

	labels := j1.Labels()
	if labels[testKey] != testVal {
		t.Errorf("expected label %s=%s", testKey, testVal)
	}

	// Test WithLabels - it replaces all labels
	testLabels := map[string]string{"team": "platform", "service": "api"}
	j2 := New("",
		Wrap(func(ctx context.Context) error { return nil }),
		WithLabels(testLabels),
	)

	labels2 := j2.Labels()
	for k, v := range testLabels {
		if labels2[k] != v {
			t.Errorf("expected label %s=%s, got %s", k, v, labels2[k])
		}
	}
}

func TestJobMultipleRuns(t *testing.T) {
	var counter int
	j := New("", func(ctx Context) error {
		counter++
		time.Sleep(100 * time.Millisecond)
		return nil
	})

	// First run
	if !j.Run(context.Background(), nil) {
		t.Error("first run should return true")
	}

	time.Sleep(10 * time.Millisecond) // Give it time to start

	// Try to run again while still executing
	if j.Run(context.Background(), nil) {
		t.Error("second run should return false while executing")
	}

	j.Wait()

	// Run again after completion
	if !j.Run(context.Background(), nil) {
		t.Error("run after completion should return true")
	}

	j.Wait()

	if counter != 2 {
		t.Errorf("expected 2 executions, got %d", counter)
	}
}

func TestJobCancelNotRunning(t *testing.T) {
	j := New("", Wrap(func(ctx context.Context) error {
		return nil
	}))

	// Try to cancel before running
	if j.Cancel() {
		t.Error("cancel should return false when job is not running")
	}

	j.Run(context.Background(), nil)
	j.Wait()

	// Try to cancel after completion
	if j.Cancel() {
		t.Error("cancel should return false when job is done")
	}
}

func TestJobPanicWithNonError(t *testing.T) {
	j := New("", Wrap(func(ctx context.Context) error {
		panic("string panic")
	}))

	j.Run(context.Background(), nil)
	j.Wait()

	if j.Err() == nil {
		t.Fatal("job should have error after panic")
	}
	if !j.IsState(StateFailed) {
		t.Errorf("job should be in failed state, got %s", j.State())
	}
}

func TestJobContextInterface(t *testing.T) {
	testID := "context-test"
	j := New(testID, func(ctx Context) error {
		// Test Context methods
		if ctx.ID() != testID {
			return errors.New("ID mismatch")
		}
		if ctx.Context() == nil {
			return errors.New("Context should not be nil")
		}
		if ctx.Logger() == nil {
			return errors.New("Logger should not be nil")
		}
		if ctx.Labels() == nil {
			return errors.New("Labels should not be nil")
		}
		return nil
	})

	j.Run(context.Background(), nil)
	j.Wait()

	if j.Err() != nil {
		t.Fatalf("context interface test failed: %v", j.Err())
	}
}

func TestJobExecutionTimeWhileRunning(t *testing.T) {
	j := New("", Wrap(func(ctx context.Context) error {
		time.Sleep(200 * time.Millisecond)
		return nil
	}))

	j.Run(context.Background(), nil)
	time.Sleep(50 * time.Millisecond)

	// Get execution time while job is still running
	execTime := j.ExecutionTime()
	if execTime < 50*time.Millisecond {
		t.Error("execution time should be at least 50ms while running")
	}

	j.Wait()
}

func TestUtilIsDone(t *testing.T) {
	tests := []struct {
		state    State
		expected bool
	}{
		{StateCreated, false},
		{StateRunning, false},
		{StateCanceling, false},
		{StateSucceeded, true},
		{StateFailed, true},
		{StateCanceled, true},
	}

	for _, tt := range tests {
		result := IsDone(tt.state)
		if result != tt.expected {
			t.Errorf("IsDone(%s) = %v, expected %v", tt.state, result, tt.expected)
		}
	}
}

func TestUtilIsPending(t *testing.T) {
	tests := []struct {
		state    State
		expected bool
	}{
		{StateCreated, false},
		{StateRunning, true},
		{StateCanceling, true},
		{StateSucceeded, false},
		{StateFailed, false},
		{StateCanceled, false},
	}

	for _, tt := range tests {
		result := IsPending(tt.state)
		if result != tt.expected {
			t.Errorf("IsPending(%s) = %v, expected %v", tt.state, result, tt.expected)
		}
	}
}

func TestJobWithCustomLogger(t *testing.T) {
	customLogger := log.Default.With()
	j := New("", Wrap(func(ctx context.Context) error {
		return nil
	}), WithLogger(customLogger))

	j.Run(context.Background(), nil)
	j.Wait()

	if j.Err() != nil {
		t.Fatalf("job with custom logger failed: %v", j.Err())
	}
}

func TestJobStatsWithRunningJob(t *testing.T) {
	j := New("", Wrap(func(ctx context.Context) error {
		time.Sleep(200 * time.Millisecond)
		return nil
	}))

	j.Run(context.Background(), nil)
	time.Sleep(50 * time.Millisecond)

	// Get stats while job is running
	stats := j.Stats()
	if stats == nil {
		t.Fatal("Stats should not be nil")
	}
	if stats.State != string(StateRunning) {
		t.Errorf("expected state %s, got %s", StateRunning, stats.State)
	}
	if stats.ExecutionTime < 50*time.Millisecond {
		t.Error("execution time should be at least 50ms")
	}

	j.Wait()
}

func TestJobKey(t *testing.T) {
	testID := "key-test"
	j := newJob(testID, Wrap(func(ctx context.Context) error {
		return nil
	}))

	if j.Key() != testID {
		t.Errorf("expected Key() to return %s, got %s", testID, j.Key())
	}
}
