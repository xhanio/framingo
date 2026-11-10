package task

import (
	"slices"
	"sync"
	"testing"

	"github.com/xhanio/framingo/pkg/utils/job"
)

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
	s := startedScheduler(t, MaxConcurrency(1))
	// ran has run before, so its job is no longer new; older is created before newer
	ran := &Task{Job: recorded("ran"), Manual: true, Priority: 1}
	older := &Task{Job: recorded("older"), Manual: true, Priority: 3}
	newer := &Task{Job: recorded("newer"), Manual: true, Priority: 3}
	fresh := &Task{Job: recorded("fresh"), Manual: true, Priority: 2}
	if err := s.Add(ran, older, newer, fresh); err != nil {
		t.Fatal(err)
	}
	if err := s.Trigger(ran.ID(), nil); err != nil {
		t.Fatal(err)
	}
	eventually(t, "ran to run once", func() bool { return ran.Job.IsState(job.StateSucceeded) })

	started, release := make(chan struct{}), make(chan struct{})
	blocker := &Task{Job: holdJob("blocker", started, release), Manual: true}
	held := &Task{Job: recorded("held"), Manual: true}
	if err := s.Add(blocker, held); err != nil {
		t.Fatal(err)
	}
	if err := s.Trigger(blocker.ID(), nil); err != nil {
		t.Fatal(err)
	}
	<-started
	// the fetching goroutine takes this one and holds it until a worker is free
	if err := s.Trigger(held.ID(), nil); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the fetching goroutine to hold a task", s.pq.IsEmpty)
	for _, tk := range []*Task{fresh, newer, older, ran} {
		if err := s.Trigger(tk.ID(), nil); err != nil {
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
