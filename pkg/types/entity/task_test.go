package entity

import (
	"testing"
)

// go vet's copylocks check flags this if Task gains state.
func TestTaskIsAPlainValue(t *testing.T) {
	a := Task{Schedule: "@daily", Priority: 3}
	b := a
	b.Priority = 4
	if a.Priority != 3 {
		t.Fatal("changing a copy changed the original")
	}
}
