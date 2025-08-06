package maputil

import "testing"

func TestSetAdd(t *testing.T) {
	tests := []struct {
		name     string
		elements []string
	}{
		{
			name:     "single element",
			elements: []string{"a"},
		},
		{
			name:     "multiple elements",
			elements: []string{"a", "b", "c"},
		},
		{
			name:     "duplicate elements",
			elements: []string{"a", "a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := make(Set[string])
			for _, elem := range tt.elements {
				s.Add(elem)
			}
			
			// Check unique elements are present
			unique := make(map[string]bool)
			for _, elem := range tt.elements {
				unique[elem] = true
			}
			
			if len(s) != len(unique) {
				t.Errorf("Set.Add() set length = %v, want %v", len(s), len(unique))
			}
			
			for elem := range unique {
				if !s.Has(elem) {
					t.Errorf("Set.Add() element %v not found in set", elem)
				}
			}
		})
	}
}

func TestSetRemove(t *testing.T) {
	tests := []struct {
		name           string
		initialElements []string
		removeElement   string
		shouldExist     bool
	}{
		{
			name:           "remove existing element",
			initialElements: []string{"a", "b", "c"},
			removeElement:   "b",
			shouldExist:     false,
		},
		{
			name:           "remove non-existing element",
			initialElements: []string{"a", "b", "c"},
			removeElement:   "d",
			shouldExist:     false,
		},
		{
			name:           "remove from empty set",
			initialElements: []string{},
			removeElement:   "a",
			shouldExist:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := make(Set[string])
			for _, elem := range tt.initialElements {
				s.Add(elem)
			}
			
			s.Remove(tt.removeElement)
			
			if s.Has(tt.removeElement) != tt.shouldExist {
				t.Errorf("Set.Remove() element %v existence = %v, want %v", 
					tt.removeElement, s.Has(tt.removeElement), tt.shouldExist)
			}
		})
	}
}

func TestSetHas(t *testing.T) {
	tests := []struct {
		name     string
		elements []string
		check    string
		expected bool
	}{
		{
			name:     "element exists",
			elements: []string{"a", "b", "c"},
			check:    "b",
			expected: true,
		},
		{
			name:     "element does not exist",
			elements: []string{"a", "b", "c"},
			check:    "d",
			expected: false,
		},
		{
			name:     "empty set",
			elements: []string{},
			check:    "a",
			expected: false,
		},
		{
			name:     "check empty string",
			elements: []string{"", "a", "b"},
			check:    "",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := make(Set[string])
			for _, elem := range tt.elements {
				s.Add(elem)
			}
			
			result := s.Has(tt.check)
			if result != tt.expected {
				t.Errorf("Set.Has() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSetIntegration(t *testing.T) {
	s := make(Set[int])
	
	// Test adding elements
	s.Add(1)
	s.Add(2)
	s.Add(3)
	s.Add(1) // duplicate
	
	if len(s) != 3 {
		t.Errorf("Set length = %v, want %v", len(s), 3)
	}
	
	// Test checking elements
	if !s.Has(1) || !s.Has(2) || !s.Has(3) {
		t.Error("Set should contain elements 1, 2, 3")
	}
	
	if s.Has(4) {
		t.Error("Set should not contain element 4")
	}
	
	// Test removing elements
	s.Remove(2)
	if s.Has(2) {
		t.Error("Set should not contain element 2 after removal")
	}
	
	if len(s) != 2 {
		t.Errorf("Set length after removal = %v, want %v", len(s), 2)
	}
	
	// Test removing non-existing element
	s.Remove(5)
	if len(s) != 2 {
		t.Errorf("Set length after removing non-existing element = %v, want %v", len(s), 2)
	}
}