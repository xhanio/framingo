package maputil

import (
	"reflect"
	"testing"
)

func TestCopyKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]int
		expected map[string]int
	}{
		{
			name:     "empty map",
			input:    map[string]int{},
			expected: map[string]int{},
		},
		{
			name:     "single element",
			input:    map[string]int{"a": 1},
			expected: map[string]int{"a": 1},
		},
		{
			name:     "multiple elements",
			input:    map[string]int{"a": 1, "b": 2, "c": 3},
			expected: map[string]int{"a": 1, "b": 2, "c": 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CopyKeys(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("CopyKeys() = %v, want %v", result, tt.expected)
			}
			// Ensure it's a copy, not the same reference
			if len(tt.input) > 0 && &result == &tt.input {
				t.Error("CopyKeys() should return a new map, not the same reference")
			}
		})
	}
}

func TestDiffKeys(t *testing.T) {
	tests := []struct {
		name           string
		from           map[string]int
		to             map[string]int
		expectedCreate map[string]int
		expectedDelete map[string]int
	}{
		{
			name:           "both empty",
			from:           map[string]int{},
			to:             map[string]int{},
			expectedCreate: map[string]int{},
			expectedDelete: map[string]int{},
		},
		{
			name:           "from empty, to has elements",
			from:           map[string]int{},
			to:             map[string]int{"a": 1, "b": 2},
			expectedCreate: map[string]int{"a": 1, "b": 2},
			expectedDelete: map[string]int{},
		},
		{
			name:           "from has elements, to empty",
			from:           map[string]int{"a": 1, "b": 2},
			to:             map[string]int{},
			expectedCreate: map[string]int{},
			expectedDelete: map[string]int{"a": 1, "b": 2},
		},
		{
			name:           "partial overlap",
			from:           map[string]int{"a": 1, "b": 2},
			to:             map[string]int{"b": 2, "c": 3},
			expectedCreate: map[string]int{"c": 3},
			expectedDelete: map[string]int{"a": 1},
		},
		{
			name:           "identical maps",
			from:           map[string]int{"a": 1, "b": 2},
			to:             map[string]int{"a": 1, "b": 2},
			expectedCreate: map[string]int{},
			expectedDelete: map[string]int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			create, delete := DiffKeys(tt.from, tt.to)
			if !reflect.DeepEqual(create, tt.expectedCreate) {
				t.Errorf("DiffKeys() create = %v, want %v", create, tt.expectedCreate)
			}
			if !reflect.DeepEqual(delete, tt.expectedDelete) {
				t.Errorf("DiffKeys() delete = %v, want %v", delete, tt.expectedDelete)
			}
		})
	}
}

func TestIn(t *testing.T) {
	tests := []struct {
		name     string
		m        map[string]int
		keys     []string
		expected bool
	}{
		{
			name:     "empty map, no keys",
			m:        map[string]int{},
			keys:     []string{},
			expected: true,
		},
		{
			name:     "empty map, with keys",
			m:        map[string]int{},
			keys:     []string{"a"},
			expected: false,
		},
		{
			name:     "single key exists",
			m:        map[string]int{"a": 1, "b": 2},
			keys:     []string{"a"},
			expected: true,
		},
		{
			name:     "single key not exists",
			m:        map[string]int{"a": 1, "b": 2},
			keys:     []string{"c"},
			expected: false,
		},
		{
			name:     "all keys exist",
			m:        map[string]int{"a": 1, "b": 2, "c": 3},
			keys:     []string{"a", "b", "c"},
			expected: true,
		},
		{
			name:     "some keys exist",
			m:        map[string]int{"a": 1, "b": 2},
			keys:     []string{"a", "c"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := In(tt.m, tt.keys...)
			if result != tt.expected {
				t.Errorf("In() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestKeys(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]int
	}{
		{
			name: "empty map",
			m:    map[string]int{},
		},
		{
			name: "single element",
			m:    map[string]int{"a": 1},
		},
		{
			name: "multiple elements",
			m:    map[string]int{"a": 1, "b": 2, "c": 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Keys(tt.m)
			if len(result) != len(tt.m) {
				t.Errorf("Keys() length = %v, want %v", len(result), len(tt.m))
			}

			// Check all keys are present
			keyMap := make(map[string]bool)
			for _, key := range result {
				keyMap[key] = true
			}

			for expectedKey := range tt.m {
				if !keyMap[expectedKey] {
					t.Errorf("Keys() missing key %v", expectedKey)
				}
			}
		})
	}
}

func TestValues(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]int
	}{
		{
			name: "empty map",
			m:    map[string]int{},
		},
		{
			name: "single element",
			m:    map[string]int{"a": 1},
		},
		{
			name: "multiple elements",
			m:    map[string]int{"a": 1, "b": 2, "c": 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Values(tt.m)
			if len(result) != len(tt.m) {
				t.Errorf("Values() length = %v, want %v", len(result), len(tt.m))
			}

			// Check all values are present
			valueMap := make(map[int]bool)
			for _, value := range result {
				valueMap[value] = true
			}

			for _, expectedValue := range tt.m {
				if !valueMap[expectedValue] {
					t.Errorf("Values() missing value %v", expectedValue)
				}
			}
		})
	}
}
