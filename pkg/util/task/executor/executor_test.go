package executor

import (
	"context"
	"strings"
	"testing"
	"time"

	"xhanio/framingo/pkg/util/errors"
	"xhanio/framingo/pkg/util/task"
)

func TestTaskRetry(t *testing.T) {
	tt := task.New("", task.Wrap(func(ctx context.Context) error {
		for {
			select {
			case <-ctx.Done():
				if ctx.Err() == context.DeadlineExceeded {
					return errors.Newf("task timed out")
				} else {
					return errors.Newf("task canceled")
				}
			case <-time.After(2 * time.Second):
				return errors.Newf("error occurred")
			}
		}
	}))

	t.Logf("task id is %s", tt.ID())
	te := New(tt, WithRetry(2, 1*time.Second))
	err := te.Start(context.Background())
	if err == nil {
		t.Fatal("failed task completed without error")
	}
	if !strings.Contains(err.Error(), "error occurred") {
		t.Fatalf("failed task completed with incorrect error: %v", tt.Err())
	}
}

func TestTaskTimedOut(t *testing.T) {
	tt := task.New("", task.Wrap(func(ctx context.Context) error {
		for {
			select {
			case <-ctx.Done():
				if ctx.Err() == context.DeadlineExceeded {
					return errors.Newf("task timed out")
				} else {
					return errors.Newf("task canceled")
				}
			case <-time.After(2 * time.Second):
				return nil
			}
		}
	}))
	t.Logf("task id is %s", tt.ID())
	te := New(tt, WithTimeout(1*time.Second))
	err := te.Start(context.Background())
	if err == nil || err.Error() != "task timed out" {
		t.Fatal("task completed without error or with incorrect err message")
	}
}

func TestTaskCanceled(t *testing.T) {
	tt := task.New("", task.Wrap(func(ctx context.Context) error {
		for {
			select {
			case <-ctx.Done():
				if ctx.Err() == context.DeadlineExceeded {
					return errors.Newf("task timed out")
				} else {
					return errors.Newf("task canceled")
				}
			case <-time.After(4 * time.Second):
				return nil
			}
		}
	}))
	t.Logf("task id is %s", tt.ID())
	te := New(tt, WithTimeout(3*time.Second))
	go func() {
		<-time.After(2 * time.Second)
		tt.Cancel()
	}()
	err := te.Start(context.Background())
	if err == nil || !tt.IsState(task.StateCanceled) {
		t.Fatalf("task completed without error or with incorrect task state %s and error %s", tt.State(), tt.Err().Error())
	}
}
