package envutil

import (
	"strings"

	"github.com/xhanio/framingo/pkg/utils/paramutil"
)

func EnvPrefix(name string) string {
	pn := name
	pn = strings.ReplaceAll(pn, " ", "_")
	pn = strings.ReplaceAll(pn, "-", "_")
	pn = strings.ToUpper(pn)
	return pn
}

// This function merges environment entry sets left to right, so a later set
// overrides an earlier one. Entries use the KEY=VALUE form, split on the first
// "=". A repeated key keeps its first-seen position and takes its last-seen
// value, so Merge([]string{"A=1"}, []string{"A=2"}) returns ["A=2"].
//
// An entry with no "=" - or with nothing before it, as the =C: entries of a
// Windows environment - is passed through unchanged in place and is never used
// as a merge key. Keys are case sensitive.
//
// The result is newly allocated and never aliases an input.
// It has a time complexity of O(n) in the total number of entries.
func Merge(sets ...[]string) []string {
	entries := paramutil.NewEntries[string]()
	for _, set := range sets {
		paramutil.Env.ParseTokensInto(entries, set)
	}
	return paramutil.Env.RenderTokens(entries)
}
