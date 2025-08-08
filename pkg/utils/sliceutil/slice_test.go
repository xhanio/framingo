package sliceutil

import (
	"reflect"
	"testing"
)

func TestIn(t *testing.T) {
	tests := []struct {
		name     string
		element  int
		elements []int
		expected bool
	}{
		{
			name:     "element exists",
			element:  2,
			elements: []int{1, 2, 3},
			expected: true,
		},
		{
			name:     "element does not exist",
			element:  4,
			elements: []int{1, 2, 3},
			expected: false,
		},
		{
			name:     "empty slice",
			element:  1,
			elements: []int{},
			expected: false,
		},
		{
			name:     "single element match",
			element:  1,
			elements: []int{1},
			expected: true,
		},
		{
			name:     "single element no match",
			element:  2,
			elements: []int{1},
			expected: false,
		},
		{
			name:     "duplicate elements",
			element:  2,
			elements: []int{2, 2, 2},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := In(tt.element, tt.elements...)
			if result != tt.expected {
				t.Errorf("In() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFirst(t *testing.T) {
	tests := []struct {
		name     string
		elements []int
		expected int
	}{
		{
			name:     "first non-zero element",
			elements: []int{0, 0, 3, 4, 0},
			expected: 3,
		},
		{
			name:     "all zero elements",
			elements: []int{0, 0, 0},
			expected: 0,
		},
		{
			name:     "empty slice",
			elements: []int{},
			expected: 0,
		},
		{
			name:     "first element non-zero",
			elements: []int{1, 2, 3},
			expected: 1,
		},
		{
			name:     "single non-zero element",
			elements: []int{5},
			expected: 5,
		},
		{
			name:     "single zero element",
			elements: []int{0},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := First(tt.elements...)
			if result != tt.expected {
				t.Errorf("First() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestLast(t *testing.T) {
	tests := []struct {
		name     string
		elements []int
		expected int
	}{
		{
			name:     "last non-zero element",
			elements: []int{0, 3, 4, 0, 0},
			expected: 4,
		},
		{
			name:     "all zero elements",
			elements: []int{0, 0, 0},
			expected: 0,
		},
		{
			name:     "empty slice",
			elements: []int{},
			expected: 0,
		},
		{
			name:     "last element non-zero",
			elements: []int{1, 2, 3},
			expected: 3,
		},
		{
			name:     "single non-zero element",
			elements: []int{5},
			expected: 5,
		},
		{
			name:     "single zero element",
			elements: []int{0},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Last(tt.elements...)
			if result != tt.expected {
				t.Errorf("Last() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsDiff(t *testing.T) {
	tests := []struct {
		name     string
		a        []int
		b        []int
		expected bool
	}{
		{
			name:     "identical slices",
			a:        []int{1, 2, 3},
			b:        []int{1, 2, 3},
			expected: false,
		},
		{
			name:     "different lengths",
			a:        []int{1, 2, 3},
			b:        []int{1, 2},
			expected: true,
		},
		{
			name:     "same length different elements",
			a:        []int{1, 2, 3},
			b:        []int{1, 2, 4},
			expected: true,
		},
		{
			name:     "both empty",
			a:        []int{},
			b:        []int{},
			expected: false,
		},
		{
			name:     "one empty one not",
			a:        []int{1},
			b:        []int{},
			expected: true,
		},
		{
			name:     "different order same elements",
			a:        []int{1, 2, 3},
			b:        []int{3, 2, 1},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsDiff(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("IsDiff() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDeduplicate(t *testing.T) {
	tests := []struct {
		name     string
		elements []int
		expected []int
	}{
		{
			name:     "no duplicates",
			elements: []int{1, 2, 3},
			expected: []int{1, 2, 3},
		},
		{
			name:     "with duplicates",
			elements: []int{1, 2, 2, 3, 1},
			expected: []int{1, 2, 3},
		},
		{
			name:     "all duplicates",
			elements: []int{1, 1, 1},
			expected: []int{1},
		},
		{
			name:     "empty slice",
			elements: []int{},
			expected: nil, // Deduplicate returns nil for empty input
		},
		{
			name:     "single element",
			elements: []int{1},
			expected: []int{1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Deduplicate(tt.elements...)
			if (result == nil && tt.expected != nil) || (result != nil && tt.expected == nil) ||
				(result != nil && tt.expected != nil && !reflect.DeepEqual(result, tt.expected)) {
				t.Errorf("Deduplicate() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestRemove(t *testing.T) {
	tests := []struct {
		name     string
		target   int
		elements []int
		expected []int
	}{
		{
			name:     "remove existing element",
			target:   2,
			elements: []int{1, 2, 3, 2, 4},
			expected: []int{1, 3, 4},
		},
		{
			name:     "remove non-existing element",
			target:   5,
			elements: []int{1, 2, 3},
			expected: []int{1, 2, 3},
		},
		{
			name:     "remove all elements",
			target:   1,
			elements: []int{1, 1, 1},
			expected: nil, // Remove returns nil for empty result
		},
		{
			name:     "empty slice",
			target:   1,
			elements: []int{},
			expected: nil, // Remove returns nil for empty input
		},
		{
			name:     "single element match",
			target:   1,
			elements: []int{1},
			expected: nil, // Remove returns nil for empty result
		},
		{
			name:     "single element no match",
			target:   2,
			elements: []int{1},
			expected: []int{1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Remove(tt.target, tt.elements...)
			if (result == nil && tt.expected != nil) || (result != nil && tt.expected == nil) ||
				(result != nil && tt.expected != nil && !reflect.DeepEqual(result, tt.expected)) {
				t.Errorf("Remove() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCopy(t *testing.T) {
	tests := []struct {
		name   string
		source []int
	}{
		{
			name:   "non-empty slice",
			source: []int{1, 2, 3},
		},
		{
			name:   "empty slice",
			source: []int{},
		},
		{
			name:   "single element",
			source: []int{42},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Copy(tt.source)

			// Check values are identical
			if !reflect.DeepEqual(result, tt.source) {
				t.Errorf("Copy() = %v, want %v", result, tt.source)
			}

			// Check it's a different slice (not same reference)
			if len(tt.source) > 0 {
				if &result[0] == &tt.source[0] {
					t.Error("Copy() should return a new slice, not the same reference")
				}

				// Modify original and ensure copy is unchanged
				original := make([]int, len(tt.source))
				copy(original, tt.source)

				if len(tt.source) > 0 {
					tt.source[0] = -999
					if !reflect.DeepEqual(result, original) {
						t.Error("Copy() result should be independent of original slice modifications")
					}
				}
			}
		})
	}
}

func TestChanges(t *testing.T) {
	tests := []struct {
		name         string
		from         []int
		to           []int
		expectedAdd  []int
		expectedRemove []int
	}{
		{
			name:         "identical slices",
			from:         []int{1, 2, 3},
			to:           []int{1, 2, 3},
			expectedAdd:  []int{},
			expectedRemove: []int{},
		},
		{
			name:         "completely different",
			from:         []int{1, 2},
			to:           []int{3, 4},
			expectedAdd:  []int{3, 4},
			expectedRemove: []int{1, 2},
		},
		{
			name:         "from is subset of to",
			from:         []int{1, 2},
			to:           []int{1, 2, 3, 4},
			expectedAdd:  []int{3, 4},
			expectedRemove: []int{},
		},
		{
			name:         "to is subset of from",
			from:         []int{1, 2, 3, 4},
			to:           []int{1, 2},
			expectedAdd:  []int{},
			expectedRemove: []int{3, 4},
		},
		{
			name:         "with duplicates in from",
			from:         []int{1, 1, 2},
			to:           []int{1, 3},
			expectedAdd:  []int{3},
			expectedRemove: []int{1, 2},
		},
		{
			name:         "with duplicates in to",
			from:         []int{1, 3},
			to:           []int{1, 1, 2},
			expectedAdd:  []int{1, 2},
			expectedRemove: []int{3},
		},
		{
			name:         "both empty",
			from:         []int{},
			to:           []int{},
			expectedAdd:  []int{},
			expectedRemove: []int{},
		},
		{
			name:         "from empty",
			from:         []int{},
			to:           []int{1, 2},
			expectedAdd:  []int{1, 2},
			expectedRemove: []int{},
		},
		{
			name:         "to empty",
			from:         []int{1, 2},
			to:           []int{},
			expectedAdd:  []int{},
			expectedRemove: []int{1, 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			add, remove := Changes(tt.from, tt.to)

			// Sort both expected and actual results for comparison since order may vary
			if !slicesEqualIgnoreOrder(add, tt.expectedAdd) {
				t.Errorf("Changes() add = %v, want %v", add, tt.expectedAdd)
			}
			if !slicesEqualIgnoreOrder(remove, tt.expectedRemove) {
				t.Errorf("Changes() remove = %v, want %v", remove, tt.expectedRemove)
			}
		})
	}
}

func slicesEqualIgnoreOrder(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}

	countA := make(map[int]int)
	countB := make(map[int]int)

	for _, v := range a {
		countA[v]++
	}
	for _, v := range b {
		countB[v]++
	}

	return reflect.DeepEqual(countA, countB)
}
