package envutil

import (
	"reflect"
	"testing"
)

func TestMerge(t *testing.T) {
	tests := []struct {
		name     string
		sets     [][]string
		expected []string
	}{
		{
			name:     "last value wins",
			sets:     [][]string{{"A=1"}, {"A=2"}},
			expected: []string{"A=2"},
		},
		{
			name:     "duplicate within a single set",
			sets:     [][]string{{"A=1", "A=2"}},
			expected: []string{"A=2"},
		},
		{
			name:     "repeated key keeps first-seen position",
			sets:     [][]string{{"A=1", "B=2"}, {"A=3"}},
			expected: []string{"A=3", "B=2"},
		},
		{
			name:     "distinct keys preserved in order",
			sets:     [][]string{{"B=1", "A=2"}},
			expected: []string{"B=1", "A=2"},
		},
		{
			name:     "empty value",
			sets:     [][]string{{"A=1"}, {"A="}},
			expected: []string{"A="},
		},
		{
			name:     "value containing equals splits on first only",
			sets:     [][]string{{"A=x=y"}},
			expected: []string{"A=x=y"},
		},
		{
			name:     "entry without equals passes through in place",
			sets:     [][]string{{"A=1", "PATH", "B=2"}},
			expected: []string{"A=1", "PATH", "B=2"},
		},
		{
			name:     "entry without equals is never a merge key",
			sets:     [][]string{{"PATH"}, {"PATH"}},
			expected: []string{"PATH", "PATH"},
		},
		{
			name:     "keys are case sensitive",
			sets:     [][]string{{"a=1"}, {"A=2"}},
			expected: []string{"a=1", "A=2"},
		},
		{
			name:     "no sets",
			sets:     nil,
			expected: []string{},
		},
		{
			name:     "empty sets",
			sets:     [][]string{{}, {}},
			expected: []string{},
		},
		{
			name:     "single set",
			sets:     [][]string{{"A=1", "B=2"}},
			expected: []string{"A=1", "B=2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Merge(tt.sets...)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Merge() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMergeDoesNotMutateInput(t *testing.T) {
	a := []string{"A=1", "B=2"}
	b := []string{"A=3"}

	result := Merge(a, b)

	if !reflect.DeepEqual(a, []string{"A=1", "B=2"}) {
		t.Errorf("Merge() mutated first input = %v, want [A=1 B=2]", a)
	}
	if !reflect.DeepEqual(b, []string{"A=3"}) {
		t.Errorf("Merge() mutated second input = %v, want [A=3]", b)
	}

	// Mutating the result must not write back into any input.
	result[0] = "MUTATED"
	if a[0] != "A=1" {
		t.Errorf("Merge() result aliases first input, a[0] = %v, want A=1", a[0])
	}
}
