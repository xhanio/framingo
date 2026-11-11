package task

import (
	"slices"
	"sync"
	"testing"

	"github.com/xhanio/framingo/pkg/structs/staque"
	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/job"
)

func TestQueueOrder(t *testing.T) {
	pq := staque.NewPriority(staque.WithLessFunc(queueOrder))
	queued := func(id string, priority int, seq uint64) *registration {
		tk := &entity.Task{Job: job.New(id, func(job.Context) error { return nil })}
		return &registration{task: tk, priority: priority, seq: seq}
	}
	pq.Push(
		queued("later", 1, 3),
		queued("b", 1, 2),
		queued("urgent", 0, 4),
		queued("a", 1, 2),
	)
	var order []string
	for {
		reg, err := pq.Pop()
		if err != nil {
			break
		}
		order = append(order, reg.Key())
	}
	// the more urgent, then the earlier queued, then by ID
	if want := []string{"urgent", "a", "b", "later"}; !slices.Equal(order, want) {
		t.Fatalf("order = %q, want %q", order, want)
	}
}

func TestQueueTakesTheMoreUrgentThenTheEarlierQueued(t *testing.T) {
	var (
		mu    sync.Mutex
		order []string
	)
	recorded := func(id string) job.Job {
		return job.New(id, func(job.Context) error {
			mu.Lock()
			defer mu.Unlock()
			order = append(order, id)
			return nil
		})
	}
	m := startedManager(t, MaxConcurrency(1))
	// ran has run before, so its job is no longer new; older is created before newer
	ran := &entity.Task{Job: recorded("ran"), Manual: true, Priority: 1}
	older := &entity.Task{Job: recorded("older"), Manual: true, Priority: 3}
	newer := &entity.Task{Job: recorded("newer"), Manual: true, Priority: 3}
	fresh := &entity.Task{Job: recorded("fresh"), Manual: true, Priority: 2}
	if err := m.Add(ran, older, newer, fresh); err != nil {
		t.Fatal(err)
	}
	if err := m.Trigger(ran.ID(), nil); err != nil {
		t.Fatal(err)
	}
	eventually(t, "ran to run once", func() bool { return ran.Job.IsState(job.StateSucceeded) })

	started, release := make(chan struct{}), make(chan struct{})
	blocker := &entity.Task{Job: holdJob("blocker", started, release), Manual: true}
	held := &entity.Task{Job: recorded("held"), Manual: true}
	if err := m.Add(blocker, held); err != nil {
		t.Fatal(err)
	}
	if err := m.Trigger(blocker.ID(), nil); err != nil {
		t.Fatal(err)
	}
	<-started
	if err := m.Trigger(held.ID(), nil); err != nil {
		t.Fatal(err)
	}
	for _, tk := range []*entity.Task{fresh, newer, older, ran} {
		if err := m.Trigger(tk.ID(), nil); err != nil {
			t.Fatal(err)
		}
	}
	close(release)
	eventually(t, "every task to run", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(order) == 6
	})
	mu.Lock()
	defer mu.Unlock()
	if want := []string{"ran", "held", "ran", "fresh", "newer", "older"}; !slices.Equal(order, want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
}
