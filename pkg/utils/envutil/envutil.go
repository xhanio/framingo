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
// An entry with no "=" is passed through unchanged in place and is never used
// as a merge key. Keys are case sensitive.
//
// The result is newly allocated and never aliases an input.
// It has a time complexity of O(n) in the total number of entries.
func Merge(sets ...[]string) []string {
	entries := paramutil.NewEntries[string]()
	for _, set := range sets {
		for _, entry := range set {
			key, value, ok := strings.Cut(entry, "=")
			if !ok {
				entries.AddRaw(entry)
				continue
			}
			entries.Set(key, value)
		}
	}
	result := make([]string, 0, entries.Len())
	for _, e := range entries.List() {
		if !e.Keyed {
			result = append(result, e.Raw)
			continue
		}
		result = append(result, e.Key+"="+e.Value)
	}
	return result
}
