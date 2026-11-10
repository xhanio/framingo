package task

import "testing"

// A Task is configuration, so it copies like any value; go vet's copylocks check flags this
// if the manager keeps state in it again.
func TestTaskIsAPlainValue(t *testing.T) {
	a := Task{Schedule: "@daily", Priority: 3}
	b := a
	b.Priority = 4
	if a.Priority != 3 {
		t.Fatal("changing a copy changed the original")
	}
}
