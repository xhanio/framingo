package staque

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

type testPriorityItem struct {
	key      string
	priority int
}

func (t *testPriorityItem) Key() string {
	return t.key
}

func (t *testPriorityItem) Priority() int {
	return t.priority
}

func (t *testPriorityItem) SetPriority(priority int) {
	t.priority = priority
}

func TestNewPriority(t *testing.T) {
	pq := NewPriority[*testPriorityItem]()
	if pq == nil {
		t.Error("NewPriority should return a non-nil priority queue")
	}
	if !pq.IsEmpty() {
		t.Error("New priority queue should be empty")
	}
	if pq.Length() != 0 {
		t.Error("New priority queue should have length 0")
	}
}

func TestNewPriorityWithOptions(t *testing.T) {
	customLessFunc := func(a, b *testPriorityItem) bool {
		return a.Priority() > b.Priority() // Reverse order
	}

	pq := NewPriority(WithLessFunc(customLessFunc))
	if pq == nil {
		t.Error("NewPriority with options should return a non-nil priority queue")
	}

	// Test that custom less function is applied
	item1 := &testPriorityItem{key: "test1", priority: 5}
	item2 := &testPriorityItem{key: "test2", priority: 10}

	pq.Push(item1, item2)

	popped, err := pq.Pop()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if popped.Priority() != 5 { // Should pop lower priority first due to custom less func
		t.Errorf("Expected priority 5, got %d", popped.Priority())
	}
}

func TestPriorityQueuePushAndPop(t *testing.T) {
	pq := NewPriority[*testPriorityItem]()

	items := []*testPriorityItem{
		{key: "low", priority: 1},
		{key: "high", priority: 10},
		{key: "medium", priority: 5},
	}

	pq.Push(items...)

	if pq.Length() != 3 {
		t.Errorf("Expected length 3, got %d", pq.Length())
	}
	if pq.IsEmpty() {
		t.Error("Queue should not be empty after push")
	}

	// Pop should return highest priority item first
	item, err := pq.Pop()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if item.Priority() != 10 {
		t.Errorf("Expected highest priority 10, got %d", item.Priority())
	}
	if item.Key() != "high" {
		t.Errorf("Expected key 'high', got '%s'", item.Key())
	}

	if pq.Length() != 2 {
		t.Errorf("Expected length 2 after pop, got %d", pq.Length())
	}
}

func TestPriorityQueuePopEmpty(t *testing.T) {
	pq := NewPriority[*testPriorityItem]()

	_, err := pq.Pop()
	if err == nil {
		t.Error("Expected error when popping from empty queue")
	}
}

func TestPriorityQueueUpdate(t *testing.T) {
	pq := NewPriority[*testPriorityItem]()

	item := &testPriorityItem{key: "test", priority: 5}
	pq.Push(item)

	// Update priority
	updatedItem := &testPriorityItem{key: "test", priority: 15}
	err := pq.Update(updatedItem)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Check that the priority was updated
	items := pq.Items()
	if len(items) != 1 {
		t.Errorf("Expected 1 item, got %d", len(items))
	}
	if items[0].Priority() != 15 {
		t.Errorf("Expected updated priority 15, got %d", items[0].Priority())
	}
}

func TestPriorityQueueUpdateNonExistent(t *testing.T) {
	pq := NewPriority[*testPriorityItem]()

	item := &testPriorityItem{key: "nonexistent", priority: 5}
	err := pq.Update(item)
	if err == nil {
		t.Error("Expected error when updating non-existent item")
	}
}

func TestPriorityQueueRemove(t *testing.T) {
	pq := NewPriority[*testPriorityItem]()

	items := []*testPriorityItem{
		{key: "item1", priority: 1},
		{key: "item2", priority: 2},
		{key: "item3", priority: 3},
	}
	pq.Push(items...)

	// Remove middle item
	removed, found := pq.Remove(&testPriorityItem{key: "item2", priority: 2})
	if !found {
		t.Error("Expected to find and remove item2")
	}
	if removed.Key() != "item2" {
		t.Errorf("Expected removed item key 'item2', got '%s'", removed.Key())
	}

	if pq.Length() != 2 {
		t.Errorf("Expected length 2 after removal, got %d", pq.Length())
	}
}

func TestPriorityQueueRemoveNonExistent(t *testing.T) {
	pq := NewPriority[*testPriorityItem]()

	item := &testPriorityItem{key: "item1", priority: 1}
	pq.Push(item)

	_, found := pq.Remove(&testPriorityItem{key: "nonexistent", priority: 1})
	if found {
		t.Error("Should not find non-existent item")
	}
}

func TestPriorityQueueItems(t *testing.T) {
	pq := NewPriority[*testPriorityItem]()

	items := []*testPriorityItem{
		{key: "item1", priority: 1},
		{key: "item2", priority: 2},
		{key: "item3", priority: 3},
	}
	pq.Push(items...)

	queueItems := pq.Items()
	if len(queueItems) != 3 {
		t.Errorf("Expected 3 items, got %d", len(queueItems))
	}

	// Verify all items are present
	keyMap := make(map[string]bool)
	for _, item := range queueItems {
		keyMap[item.Key()] = true
	}

	for _, expectedItem := range items {
		if !keyMap[expectedItem.Key()] {
			t.Errorf("Expected item with key '%s' not found", expectedItem.Key())
		}
	}
}

func TestPriorityQueueDuplicateKeys(t *testing.T) {
	pq := NewPriority[*testPriorityItem]()

	item1 := &testPriorityItem{key: "duplicate", priority: 5}
	item2 := &testPriorityItem{key: "duplicate", priority: 10}

	pq.Push(item1, item2)

	// Should only have one item since keys are the same
	if pq.Length() != 1 {
		t.Errorf("Expected length 1 for duplicate keys, got %d", pq.Length())
	}
}

func TestPriorityQueueConcurrency(t *testing.T) {
	pq := NewPriority[*testPriorityItem]()

	var wg sync.WaitGroup
	numGoroutines := 10
	itemsPerGoroutine := 10

	// Concurrent push operations
	for i := range numGoroutines {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := range itemsPerGoroutine {
				key := fmt.Sprintf("item_%d_%d", goroutineID, j)
				item := &testPriorityItem{key: key, priority: j}
				pq.Push(item)
			}
		}(i)
	}

	wg.Wait()

	expectedLength := numGoroutines * itemsPerGoroutine
	if pq.Length() != expectedLength {
		t.Errorf("Expected length %d after concurrent operations, got %d", expectedLength, pq.Length())
	}
}

func TestDefaultLessFunc(t *testing.T) {
	item1 := &testPriorityItem{key: "item1", priority: 5}
	item2 := &testPriorityItem{key: "item2", priority: 10}

	if !DefaultLessFunc(item1, item2) {
		t.Error("LessFunc should return true when first item has lower priority")
	}

	if DefaultLessFunc(item2, item1) {
		t.Error("LessFunc should return false when first item has higher priority")
	}

	item3 := &testPriorityItem{key: "item3", priority: 5}
	// With updated LessFunc, equal priority items are ordered by key
	if DefaultLessFunc(item1, item3) != (item1.Key() < item3.Key()) {
		t.Error("LessFunc should order by key when priorities are equal")
	}
}

func TestWithLessFuncOption(t *testing.T) {
	// Test nil less function (should not change default)
	pq := NewPriority(WithLessFunc[*testPriorityItem](nil))

	item1 := &testPriorityItem{key: "item1", priority: 5}
	item2 := &testPriorityItem{key: "item2", priority: 10}

	pq.Push(item1, item2)

	popped, err := pq.Pop()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if popped.Priority() != 10 { // Should still use default (higher priority first)
		t.Errorf("Expected priority 10, got %d", popped.Priority())
	}
}

func TestWaitTakesFromEitherEndAtOnce(t *testing.T) {
	pq := NewPriority[*testPriorityItem]()
	pq.Push(&testPriorityItem{key: "low", priority: 1}, &testPriorityItem{key: "mid", priority: 5}, &testPriorityItem{key: "high", priority: 10})
	if item, err := pq.WaitPop(context.Background()); err != nil || item.Key() != "high" {
		t.Fatalf("WaitPop = %v, %v, want high", item, err)
	}
	if item, err := pq.WaitShift(context.Background()); err != nil || item.Key() != "low" {
		t.Fatalf("WaitShift = %v, %v, want low", item, err)
	}
	if n := pq.Length(); n != 1 {
		t.Fatalf("length = %d, want 1", n)
	}
}

func TestWaitWaitsForAPush(t *testing.T) {
	for name, wait := range map[string]func(Priority[*testPriorityItem], context.Context) (*testPriorityItem, error){
		"WaitPop":   Priority[*testPriorityItem].WaitPop,
		"WaitShift": Priority[*testPriorityItem].WaitShift,
	} {
		t.Run(name, func(t *testing.T) {
			pq := NewPriority[*testPriorityItem]()
			got := make(chan *testPriorityItem, 1)
			go func() {
				item, _ := wait(pq, context.Background())
				got <- item
			}()
			select {
			case item := <-got:
				t.Fatalf("returned %v from an empty queue", item)
			case <-time.After(50 * time.Millisecond):
			}
			pq.Push(&testPriorityItem{key: "pushed"})
			select {
			case item := <-got:
				if item == nil || item.Key() != "pushed" {
					t.Fatalf("returned %v, want pushed", item)
				}
			case <-time.After(time.Second):
				t.Fatal("did not return once an element was pushed")
			}
		})
	}
}

func TestWaitReturnsTheContextsErrorOnceItEnds(t *testing.T) {
	pq := NewPriority[*testPriorityItem]()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	begin := time.Now()
	if _, err := pq.WaitShift(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("WaitShift = %v, want the deadline's error", err)
	}
	if took := time.Since(begin); took < 50*time.Millisecond || took > time.Second {
		t.Fatalf("WaitShift returned after %s, want as its 50ms deadline passed", took)
	}
}

func TestWaitWithAnEndedContextTakesNothing(t *testing.T) {
	pq := NewPriority[*testPriorityItem]()
	pq.Push(&testPriorityItem{key: "kept"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := pq.WaitPop(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("WaitPop = %v, want the context's error", err)
	}
	if n := pq.Length(); n != 1 {
		t.Fatalf("length = %d, want 1: an ended wait took an element", n)
	}
}

func TestWaitersTakeEachElementOnce(t *testing.T) {
	const waiters = 8
	pq := NewPriority[*testPriorityItem]()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var (
		mu   sync.Mutex
		seen = map[string]int{}
		wg   sync.WaitGroup
	)
	for range waiters {
		wg.Add(1)
		go func() {
			defer wg.Done()
			item, err := pq.WaitShift(ctx)
			if err != nil {
				t.Errorf("WaitShift = %v", err)
				return
			}
			mu.Lock()
			seen[item.Key()]++
			mu.Unlock()
		}()
	}
	time.Sleep(50 * time.Millisecond) // the waiters wait on an empty queue
	var items []*testPriorityItem
	for i := range waiters {
		items = append(items, &testPriorityItem{key: string(rune('a' + i)), priority: i})
	}
	pq.Push(items[:waiters/2]...) // several at once, then one at a time
	for _, item := range items[waiters/2:] {
		pq.Push(item)
	}
	wg.Wait()
	if len(seen) != waiters {
		t.Fatalf("seen = %v, want %d elements", seen, waiters)
	}
	for key, n := range seen {
		if n != 1 {
			t.Fatalf("%s was taken %d times", key, n)
		}
	}
}
