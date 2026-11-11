package task

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/job"
)

func TestQuickTasksDoNotUnbalanceTheQueue(t *testing.T) {
	const n = 3000
	var done atomic.Int32
	s := newManager(MaxConcurrency(3))
	if err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Stop(context.Background()) }()
	for i := range n {
		tk := &entity.Task{Job: job.New(fmt.Sprintf("quick-%d", i), func(job.Context) error {
			done.Add(1)
			return nil
		})}
		if err := s.Add(tk); err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.Now().Add(20 * time.Second)
	for done.Load() < n && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := done.Load(); got != n {
		t.Fatalf("%d of %d quick tasks finished", got, n)
	}
}
