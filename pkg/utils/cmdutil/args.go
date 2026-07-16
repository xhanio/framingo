package cmdutil

import "strings"

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
	type slot struct {
		key   string
		raw   string
		keyed bool
	}
	type flag struct {
		inline bool
		values []string
	}
	var slots []slot
	flags := make(map[string]*flag)
	for _, set := range sets {
		var open *flag
		for _, token := range set {
			if !isFlag(token) {
				if open != nil {
					open.values = append(open.values, token)
					continue
				}
				slots = append(slots, slot{raw: token})
				continue
			}
			key := token
			var values []string
			inline := false
			if i := strings.Index(token, "="); i >= 0 {
				key = token[:i]
				values = []string{token[i+1:]}
				inline = true
			}
			f, ok := flags[key]
			if !ok {
				f = &flag{}
				flags[key] = f
				slots = append(slots, slot{key: key, keyed: true})
			}
			f.inline = inline
			f.values = values
			open = f
			if inline {
				open = nil
			}
		}
	}
	result := make([]string, 0, len(slots))
	for _, s := range slots {
		if !s.keyed {
			result = append(result, s.raw)
			continue
		}
		f := flags[s.key]
		if f.inline {
			result = append(result, s.key+"="+f.values[0])
			continue
		}
		result = append(result, s.key)
		result = append(result, f.values...)
	}
	return result
}

// This function reports whether a token is a flag rather than a positional.
// Bare "-" is a positional.
// It has a time complexity of O(1).
func isFlag(token string) bool {
	return len(token) > 1 && strings.HasPrefix(token, "-")
}
