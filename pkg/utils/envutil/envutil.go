package envutil

import "strings"

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
	type slot struct {
		key   string
		raw   string
		keyed bool
	}
	var slots []slot
	values := make(map[string]string)
	for _, set := range sets {
		for _, entry := range set {
			i := strings.Index(entry, "=")
			if i < 0 {
				slots = append(slots, slot{raw: entry})
				continue
			}
			key := entry[:i]
			if _, ok := values[key]; !ok {
				slots = append(slots, slot{key: key, keyed: true})
			}
			values[key] = entry[i+1:]
		}
	}
	result := make([]string, 0, len(slots))
	for _, s := range slots {
		if !s.keyed {
			result = append(result, s.raw)
			continue
		}
		result = append(result, s.key+"="+values[s.key])
	}
	return result
}
