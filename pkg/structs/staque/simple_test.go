package staque

import (
	"encoding/json"
	"slices"
	"sync"
	"testing"
)

type A struct {
	DataA string
}

type B struct {
	DataB int
}

func TestQueue(t *testing.T) {
	data := make(map[string]any)
	queueA := newSimple[*A](0)
	queueA.Push(&A{DataA: "aaa"})
	queueB := newSimple[*B](0)
	queueB.Push(&B{DataB: 111})
	data["A"] = queueA.data
	data["B"] = queueB.data

	b, err := json.MarshalIndent(&data, "", "  ")
	if err != nil {
		t.Error(err)
		return
	}

	t.Logf("data: %s", string(b))
	restored := make(map[string]any)
	err = json.Unmarshal(b, &restored)
	if err != nil {
		t.Error(err)
		return
	}
	restoredA, err := toQueue[*A](restored["A"])
	if err != nil {
		t.Error(err)
		return
	}
	restoredB, err := toQueue[*B](restored["B"])
	if err != nil {
		t.Error(err)
		return
	}
	objA := restoredA.MustPop()
	objB := restoredB.MustPop()
	t.Logf("DataA: %s, DataB: %d", objA.DataA, objB.DataB)
	if objA.DataA != "aaa" || objB.DataB != 111 {
		t.FailNow()
	}
}

func TestQueueFuncs(t *testing.T) {
	s := &simple[int]{}
	s.Push(1, 2, 3, 4, 5)
	n1, err := s.Shift()
	if err != nil || n1 != 1 {
		t.Error(err)
		return
	}
	n23, err := s.ShiftN(2)
	if err != nil || len(n23) != 2 || n23[0] != 2 || n23[1] != 3 {
		t.Error(err)
		return
	}
	n5, err := s.Pop()
	if err != nil || n5 != 5 {
		t.Error(err)
		return
	}
	s.Push(5, 6, 7, 8)
	n78, err := s.PopN(2)
	if err != nil || len(n78) != 2 || n78[0] != 7 || n78[1] != 8 {
		t.Error(err)
		return
	}
}

func TestQueueMaxSize(t *testing.T) {
	s := &simple[int]{maxSize: 3}
	s.Push(1, 2, 3, 4, 5, 6)
	if s.Length() != 3 {
		t.Error("length should be 3, now is", s.Length())
	}
}

func TestMultiPush(t *testing.T) {
	s := &simple[int]{}

	wg := &sync.WaitGroup{}
	for i := range 10 {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			s.Push(index)
		}(i)
	}
	wg.Wait()

	if s.Length() != 10 {
		t.Errorf("Expected Length to be 10 after concurrent Push operations, but got %d.", s.Length())
	}
}

func TestShiftN(t *testing.T) {
	s := &simple[int]{data: []int{1, 2, 3, 4, 5, 6}}

	elements1, err := s.ShiftN(2)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	expected1 := []int{1, 2}
	if !slices.Equal(elements1, expected1) {
		t.Errorf("Expected %v, but got %v", expected1, elements1)
	}

	// Second shift 2 elements
	elements2, err := s.ShiftN(2)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	expected2 := []int{3, 4}
	if !slices.Equal(elements2, expected2) {
		t.Errorf("Expected %v, but got %v", expected2, elements2)
	}

	// Ensure that the simple is updated after the shifts
	expectedQueue := []int{5, 6}
	if !slices.Equal(s.data, expectedQueue) {
		t.Errorf("Expected simple %v, but got %v", expectedQueue, s.data)
	}
}
