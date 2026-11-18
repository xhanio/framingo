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

// Merge merges left to right: a repeated key keeps its first position and its last value. Keys
// are case sensitive; entries without a key (no "=", or Windows "=C:") pass through in place.
func Merge(sets ...[]string) []string {
	entries := paramutil.NewEntries[string]()
	for _, set := range sets {
		paramutil.Env.ParseTokensInto(entries, set)
	}
	return paramutil.Env.RenderTokens(entries)
}
