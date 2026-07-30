package cmdutil

import (
	"strings"

	"github.com/xhanio/framingo/pkg/structs/kv"
)

// This function merges argument sets left to right, so a later set overrides an
// earlier one. A token longer than one character and starting with "-" is a
// flag; following non-flag tokens are its values. A repeated flag keeps its
// first-seen position and takes its last-seen value list, so
// MergeArgs([]string{"-a", "-b", "2", "-b", "3"}) returns ["-a", "-b", "3"].
//
// The --flag=value form splits on the first "=" and merges with the space form
// of the same flag; the emitted form follows the last-seen occurrence. Bare "-"
// and tokens with no flag open are positionals, passed through in place. A flag
// does not stay open across a set boundary.
//
// The result is newly allocated and never aliases an input.
// It has a time complexity of O(n) in the total number of tokens.
//
// This function infers flag arity positionally, because an argument list does
// not say which flags take a value. Two consequences:
//
//   - Repeatable flags collapse: ["-v", "-v"] returns ["-v"], and
//     ["--header", "A", "--header", "B"] returns ["--header", "B"]. Do not use
//     MergeArgs for tools whose flags accumulate.
//   - A trailing positional is absorbed as the preceding flag's value:
//     ["--verbose", "file.txt"] parses file.txt as --verbose's value. There is
//     no "--" terminator to guard against this.
func MergeArgs(sets ...[]string) []string {
	type flag struct {
		inline bool
		values []string
	}
	entries := kv.New[*flag]()
	for _, set := range sets {
		var open *flag
		for _, token := range set {
			if !isFlag(token) {
				if open != nil {
					open.values = append(open.values, token)
					continue
				}
				entries.AddRaw(token)
				continue
			}
			key := token
			var values []string
			inline := false
			if k, v, ok := strings.Cut(token, "="); ok {
				key = k
				values = []string{v}
				inline = true
			}
			f, ok := entries.Get(key)
			if !ok {
				f = &flag{}
			}
			f.inline = inline
			f.values = values
			entries.Set(key, f)
			open = f
			if inline {
				open = nil
			}
		}
	}
	result := make([]string, 0, entries.Len())
	for _, e := range entries.Entries() {
		if !e.Keyed {
			result = append(result, e.Raw)
			continue
		}
		if e.Value.inline {
			result = append(result, e.Key+"="+e.Value.values[0])
			continue
		}
		result = append(result, e.Key)
		result = append(result, e.Value.values...)
	}
	return result
}

// This function reports whether a token is a flag rather than a positional.
// Bare "-" is a positional.
// It has a time complexity of O(1).
func isFlag(token string) bool {
	return len(token) > 1 && strings.HasPrefix(token, "-")
}
