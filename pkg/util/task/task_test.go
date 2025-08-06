package task

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTaskCompleted(t *testing.T) {
	tt := New("", Wrap(func(ctx context.Context) error {
		for {
			select {
			case <-ctx.Done():
				if ctx.Err() == context.DeadlineExceeded {
					return errors.New("task timed out")
				} else {
					return errors.New("task canceled")
				}
			case <-time.After(2 * time.Second):
				return nil
			}
		}
	}))
	t.Logf("task id is %s", tt.ID())
	tt.Start(context.Background())
	tt.Wait()
	if tt.Err() != nil {
		t.Fatalf("completed task failed with error: %s", tt.Err().Error())
	}
}

func TestTaskPanicked(t *testing.T) {
	tt := New("", Wrap(func(ctx context.Context) error {
		for {
			select {
			case <-ctx.Done():
				if ctx.Err() == context.DeadlineExceeded {
					return errors.New("task timed out")
				} else {
					return errors.New("task canceled")
				}
			case <-time.After(2 * time.Second):
				panic(errors.New("?!"))
			}
		}
	}))
	t.Logf("task id is %s", tt.ID())
	tt.Start(context.Background())
	tt.Wait()
	if tt.Err() == nil || tt.Err().Error() != "?!" {
		t.Fatal("panicked task completed without error or with incorrect err message")
	}
}
