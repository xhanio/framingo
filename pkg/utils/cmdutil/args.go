package cmdutil

import "github.com/xhanio/framingo/pkg/utils/paramutil"

// MergeArgs merges left to right: a repeated flag keeps its first position and its last values
// (--f=v and --f v are the same flag). Arity is inferred, so repeatable flags collapse and a
// positional after a flag becomes its value; there is no "--" terminator.
func MergeArgs(sets ...[]string) []string {
	entries := paramutil.NewEntries[paramutil.Arg]()
	for _, set := range sets {
		paramutil.Argv.ParseInto(entries, set)
	}
	return paramutil.Argv.Render(entries)
}
