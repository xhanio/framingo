package strutil

import (
	"fmt"
	"strings"
	"testing"
)

func TestAllLetters(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "all lowercase letters",
			input:    "hello",
			expected: true,
		},
		{
			name:     "all uppercase letters",
			input:    "HELLO",
			expected: true,
		},
		{
			name:     "mixed case letters",
			input:    "Hello",
			expected: true,
		},
		{
			name:     "contains numbers",
			input:    "hello123",
			expected: false,
		},
		{
			name:     "contains spaces",
			input:    "hello world",
			expected: false,
		},
		{
			name:     "contains special characters",
			input:    "hello!",
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "only numbers",
			input:    "123",
			expected: false,
		},
		{
			name:     "single letter",
			input:    "a",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AllLetters(tt.input)
			if result != tt.expected {
				t.Errorf("AllLetters(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

type testStringer struct {
	value string
}

func (t testStringer) String() string {
	return t.value
}

func TestJoin(t *testing.T) {
	tests := []struct {
		name      string
		sep       string
		elements  []fmt.Stringer
		expected  string
	}{
		{
			name:     "join with comma",
			sep:      ",",
			elements: []fmt.Stringer{testStringer{"a"}, testStringer{"b"}, testStringer{"c"}},
			expected: "a,b,c",
		},
		{
			name:     "join with space",
			sep:      " ",
			elements: []fmt.Stringer{testStringer{"hello"}, testStringer{"world"}},
			expected: "hello world",
		},
		{
			name:     "empty separator",
			sep:      "",
			elements: []fmt.Stringer{testStringer{"a"}, testStringer{"b"}},
			expected: "ab",
		},
		{
			name:     "single element",
			sep:      ",",
			elements: []fmt.Stringer{testStringer{"single"}},
			expected: "single",
		},
		{
			name:     "empty elements",
			sep:      ",",
			elements: []fmt.Stringer{},
			expected: "",
		},
		{
			name:     "skip empty strings",
			sep:      ",",
			elements: []fmt.Stringer{testStringer{"a"}, testStringer{""}, testStringer{"b"}},
			expected: "a,b",
		},
		{
			name:     "all empty strings",
			sep:      ",",
			elements: []fmt.Stringer{testStringer{""}, testStringer{""}, testStringer{""}},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Join(tt.sep, tt.elements...)
			if result != tt.expected {
				t.Errorf("Join(%q, %v) = %q, want %q", tt.sep, tt.elements, result, tt.expected)
			}
		})
	}
}

func TestClean(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "trim spaces",
			input:    "  hello  ",
			expected: "hello",
		},
		{
			name:     "trim newlines",
			input:    "\nhello\n",
			expected: "hello",
		},
		{
			name:     "trim tabs",
			input:    "\thello\t",
			expected: "hello",
		},
		{
			name:     "trim mixed whitespace",
			input:    " \n\thello \t\n ",
			expected: "hello",
		},
		{
			name:     "no whitespace",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only whitespace",
			input:    " \n\t ",
			expected: "",
		},
		{
			name:     "internal whitespace preserved",
			input:    "  hello world  ",
			expected: "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Clean(tt.input)
			if result != tt.expected {
				t.Errorf("Clean(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRandom(t *testing.T) {
	tests := []struct {
		name    string
		charset string
		length  int
	}{
		{
			name:    "lowercase letters",
			charset: "abcdefghijklmnopqrstuvwxyz",
			length:  10,
		},
		{
			name:    "numbers",
			charset: "0123456789",
			length:  5,
		},
		{
			name:    "mixed charset",
			charset: "abc123",
			length:  8,
		},
		{
			name:    "single character",
			charset: "a",
			length:  5,
		},
		{
			name:    "zero length",
			charset: "abc",
			length:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Random(tt.charset, tt.length)
			
			// Check length
			if len(result) != tt.length {
				t.Errorf("Random() length = %v, want %v", len(result), tt.length)
			}
			
			// Check all characters are from charset
			for _, char := range result {
				if !strings.ContainsRune(tt.charset, char) {
					t.Errorf("Random() contains invalid character %c, charset: %s", char, tt.charset)
				}
			}
		})
	}
}

func TestFormatHex(t *testing.T) {
	tests := []struct {
		name      string
		num       interface{}
		uppercase bool
		expected  string
	}{
		{
			name:      "single byte lowercase",
			num:       uint8(15),
			uppercase: false,
			expected:  "0f",
		},
		{
			name:      "single byte uppercase",
			num:       uint8(15),
			uppercase: true,
			expected:  "0F",
		},
		{
			name:      "two bytes lowercase",
			num:       uint16(0x1234),
			uppercase: false,
			expected:  "12:34",
		},
		{
			name:      "two bytes uppercase",
			num:       uint16(0x1234),
			uppercase: true,
			expected:  "12:34",
		},
		{
			name:      "four bytes lowercase",
			num:       uint32(0x12345678),
			uppercase: false,
			expected:  "12:34:56:78",
		},
		{
			name:      "four bytes uppercase",
			num:       uint32(0x12345678),
			uppercase: true,
			expected:  "12:34:56:78",
		},
		{
			name:      "zero value",
			num:       uint8(0),
			uppercase: false,
			expected:  "00",
		},
		{
			name:      "odd length hex",
			num:       uint8(5),
			uppercase: false,
			expected:  "05",
		},
		{
			name:      "maximum byte value",
			num:       uint8(255),
			uppercase: false,
			expected:  "ff",
		},
		{
			name:      "maximum byte value uppercase",
			num:       uint8(255),
			uppercase: true,
			expected:  "FF",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatHex(tt.num, tt.uppercase)
			if result != tt.expected {
				t.Errorf("FormatHex(%v, %v) = %q, want %q", tt.num, tt.uppercase, result, tt.expected)
			}
		})
	}
}

func TestPrefixIn(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		prefixes []string
		expected bool
	}{
		{
			name:     "has matching prefix",
			s:        "hello world",
			prefixes: []string{"hi", "hello", "bye"},
			expected: true,
		},
		{
			name:     "no matching prefix",
			s:        "hello world",
			prefixes: []string{"hi", "bye", "good"},
			expected: false,
		},
		{
			name:     "empty prefixes",
			s:        "hello world",
			prefixes: []string{},
			expected: false,
		},
		{
			name:     "empty string with non-empty prefixes",
			s:        "",
			prefixes: []string{"hello", "hi"},
			expected: false,
		},
		{
			name:     "empty string with empty prefix",
			s:        "",
			prefixes: []string{"", "hello"},
			expected: true,
		},
		{
			name:     "exact match",
			s:        "hello",
			prefixes: []string{"hello"},
			expected: true,
		},
		{
			name:     "prefix longer than string",
			s:        "hi",
			prefixes: []string{"hello"},
			expected: false,
		},
		{
			name:     "multiple matching prefixes",
			s:        "hello world",
			prefixes: []string{"hel", "hello", "hell"},
			expected: true,
		},
		{
			name:     "case sensitive",
			s:        "Hello world",
			prefixes: []string{"hello"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PrefixIn(tt.s, tt.prefixes...)
			if result != tt.expected {
				t.Errorf("PrefixIn(%q, %v) = %v, want %v", tt.s, tt.prefixes, result, tt.expected)
			}
		})
	}
}