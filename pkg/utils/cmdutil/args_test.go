package cmdutil

import (
	"reflect"
	"testing"
)

func TestMergeArgs(t *testing.T) {
	tests := []struct {
		name     string
		sets     [][]string
		expected []string
	}{
		{
			name:     "repeated flag takes last value and keeps first position",
			sets:     [][]string{{"-a", "-b", "2", "-b", "3"}},
			expected: []string{"-a", "-b", "3"},
		},
		{
			name:     "override across sets",
			sets:     [][]string{{"-a", "-b", "2"}, {"-b", "3"}},
			expected: []string{"-a", "-b", "3"},
		},
		{
			name:     "boolean flag with no value",
			sets:     [][]string{{"-a"}, {"-a"}},
			expected: []string{"-a"},
		},
		{
			name:     "repeatable flags collapse (documented limitation)",
			sets:     [][]string{{"-v", "-v", "-v"}},
			expected: []string{"-v"},
		},
		{
			name:     "accumulating flags collapse (documented limitation)",
			sets:     [][]string{{"--header", "A", "--header", "B"}},
			expected: []string{"--header", "B"},
		},
		{
			name:     "equals form merges",
			sets:     [][]string{{"--output=json"}, {"--output=yaml"}},
			expected: []string{"--output=yaml"},
		},
		{
			name:     "equals form then space form emits space form",
			sets:     [][]string{{"--output=json"}, {"--output", "yaml"}},
			expected: []string{"--output", "yaml"},
		},
		{
			name:     "space form then equals form emits equals form",
			sets:     [][]string{{"--output", "json"}, {"--output=yaml"}},
			expected: []string{"--output=yaml"},
		},
		{
			name:     "equals form closes the flag so next token is positional",
			sets:     [][]string{{"--output=json", "file.txt"}},
			expected: []string{"--output=json", "file.txt"},
		},
		{
			name:     "value list replaced as a unit",
			sets:     [][]string{{"-b", "2", "3"}, {"-b", "9"}},
			expected: []string{"-b", "9"},
		},
		{
			name:     "multi value flag preserved when not overridden",
			sets:     [][]string{{"-b", "2", "3"}},
			expected: []string{"-b", "2", "3"},
		},
		{
			name:     "leading positionals pass through",
			sets:     [][]string{{"build", "-x"}},
			expected: []string{"build", "-x"},
		},
		{
			name:     "bare dash is a positional",
			sets:     [][]string{{"cat", "-"}},
			expected: []string{"cat", "-"},
		},
		{
			name:     "trailing positional is absorbed (documented limitation)",
			sets:     [][]string{{"--verbose", "file.txt"}},
			expected: []string{"--verbose", "file.txt"},
		},
		{
			name:     "flag does not stay open across sets",
			sets:     [][]string{{"-b"}, {"2"}},
			expected: []string{"-b", "2"},
		},
		{
			name:     "empty value in equals form",
			sets:     [][]string{{"--output="}},
			expected: []string{"--output="},
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
			name:     "single set unchanged",
			sets:     [][]string{{"-a", "-b", "2"}},
			expected: []string{"-a", "-b", "2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeArgs(tt.sets...)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("MergeArgs() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMergeArgsDoesNotMutateInput(t *testing.T) {
	a := []string{"-a", "-b", "2"}
	b := []string{"-b", "3"}

	MergeArgs(a, b)

	if !reflect.DeepEqual(a, []string{"-a", "-b", "2"}) {
		t.Errorf("MergeArgs() mutated first input = %v, want [-a -b 2]", a)
	}
	if !reflect.DeepEqual(b, []string{"-b", "3"}) {
		t.Errorf("MergeArgs() mutated second input = %v, want [-b 3]", b)
	}
}
