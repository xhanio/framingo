package planner

import "testing"

func TestBashJobsGetDistinctIDs(t *testing.T) {
	a := NewBashJob("test", nil, "true", nil)
	b := NewBashJob("test", nil, "true", nil)
	if a.ID() == b.ID() {
		t.Fatalf("two bash jobs share the ID %q; the task manager would treat them as one", a.ID())
	}
	c := NewAsyncBashJob("test", nil, "true", nil)
	d := NewAsyncBashJob("test", nil, "true", nil)
	if c.ID() == d.ID() {
		t.Fatalf("two async bash jobs share the ID %q", c.ID())
	}
}
