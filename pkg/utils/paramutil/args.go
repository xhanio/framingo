package paramutil

import (
	"slices"
	"strings"
)

// Argv reads a command line as a shell hands one over: --long and -short flags,
// their values written after an equal sign or in the tokens that follow, and
// the positionals among them.
var Argv = NewArgs("=", []string{"--", "-"})

type args struct {
	equal    string
	prefixes []string
}

// NewArgs returns the notation whose flags carry one of prefixes and take a
// value written after equal or in the tokens that follow. The prefixes are
// taken as they are then, never aliased.
func NewArgs(equal string, prefixes []string) Args {
	return &args{
		equal:    equal,
		prefixes: slices.Clone(prefixes),
	}
}

func (a *args) Parse(tokens []string) Entries[Arg] {
	entries := NewEntries[Arg]()
	a.ParseInto(entries, tokens)
	return entries
}

func (a *args) ParseInto(entries Entries[Arg], tokens []string) {
	// A flag left open at the end of one call does not take the tokens of the
	// next: what follows it there is a line of its own.
	open := ""
	for _, token := range tokens {
		if !a.names(token) {
			if open == "" {
				entries.AddRaw(token)
				continue
			}
			arg, _ := entries.Get(open)
			arg.Values = append(arg.Values, token)
			entries.Set(open, arg)
			continue
		}
		key, arg := token, Arg{}
		if k, v, ok := strings.Cut(token, a.equal); ok {
			key, arg = k, Arg{Inline: true, Values: []string{v}}
		}
		// A flag seen again keeps its position and starts its values over, so a
		// later line replaces an earlier one rather than adding to it.
		entries.Set(key, arg)
		open = key
		if arg.Inline {
			open = ""
		}
	}
}

func (a *args) Render(entries Entries[Arg]) []string {
	if entries == nil {
		return nil
	}
	out := make([]string, 0, entries.Len())
	for _, e := range entries.List() {
		if !e.Keyed {
			out = append(out, e.Raw)
			continue
		}
		if e.Value.Inline {
			var value string
			if len(e.Value.Values) > 0 {
				value = e.Value.Values[0]
			}
			out = append(out, e.Key+a.equal+value)
			continue
		}
		out = append(out, e.Key)
		out = append(out, e.Value.Values...)
	}
	return out
}

// names reports whether a token names a flag, which it does by carrying one of
// the prefixes and holding more than it. Under Argv's prefixes that leaves a
// bare "-" a positional, while "--" carries "-" and is longer than it, so it
// names a flag: there is no terminator handling.
func (a *args) names(token string) bool {
	for _, p := range a.prefixes {
		if strings.HasPrefix(token, p) && len(token) > len(p) {
			return true
		}
	}
	return false
}
