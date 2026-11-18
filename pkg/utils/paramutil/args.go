package paramutil

import (
	"slices"
	"strings"
)

var Argv = NewArgs("=", []string{"--", "-"})

type args struct {
	equal    string
	prefixes []string
}

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
		// a repeated flag keeps its position but replaces its values
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

// names: a bare "-" is a positional, but "--" names a flag; there is no terminator handling.
func (a *args) names(token string) bool {
	for _, p := range a.prefixes {
		if strings.HasPrefix(token, p) && len(token) > len(p) {
			return true
		}
	}
	return false
}
