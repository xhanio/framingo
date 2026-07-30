package cmdutil

import "github.com/xhanio/framingo/pkg/utils/paramutil"

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
	entries := paramutil.NewEntries[paramutil.Arg]()
	for _, set := range sets {
		paramutil.Argv.ParseInto(entries, set)
	}
	return paramutil.Argv.Render(entries)
}
