package kv

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// badKeyChars are the characters that tell a key apart from a fragment of a URI
// or a credential.
const badKeyChars = "/@:"

// The notations a list is commonly spelled in.
var (
	// Query reads a=b&c=d, opened by the "?" of a URL when one precedes it.
	Query = NewFormat("=", "&", WithOpener("?"))
	// Keyword reads a=b c=d, as a libpq connection string does.
	Keyword = NewFormat("=", " ")
	// Flags reads --a=b --c=d, as long command line flags do.
	Flags = NewFormat("=", " ", WithKeyPrefix("--"))
	// Comma reads a=b,c=d.
	Comma = NewFormat("=", ",")
)

type format struct {
	equal     string
	sep       string
	open      string
	keyPrefix string
}

// NewFormat returns the notation whose pairs read as key<equal>value and are
// joined by sep. Without WithOpener the whole string is pairs, and sep divides
// them from any text that precedes them.
func NewFormat(equal, sep string, opts ...FormatOption) Format {
	f := &format{
		equal: equal,
		sep:   sep,
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// opener returns the text that starts the section. A notation with no opener of
// its own reuses its separator, having nothing else to divide a prefix from the
// pairs that follow it.
func (f *format) opener() string {
	if f.open == "" {
		return f.sep
	}
	return f.open
}

func (f *format) Parse(s string) (string, List[string]) {
	if f.open != "" {
		// The section opens at the last opener, which keeps one inside a
		// password or a path out of it.
		if i := strings.LastIndex(s, f.open); i >= 0 {
			prefix, tail := s[:i+len(f.open)], s[i+len(f.open):]
			if tail == "" {
				return prefix, New[string]()
			}
			if entries, ok := f.parse(strings.Split(tail, f.sep)); ok {
				return prefix, entries
			}
		}
		// No opener, or what followed it was not a section. The string may
		// still be pairs and nothing else, so read it as one that never opens.
	}
	// Every token is a pair, so scan back from the end for as long as they read
	// as one and leave the rest as prefix.
	tokens := strings.Split(s, f.sep)
	i := len(tokens)
	for ; i > 0; i-- {
		if _, _, ok := f.cut(tokens[i-1]); !ok {
			break
		}
	}
	entries, _ := f.parse(tokens[i:])
	return strings.Join(tokens[:i], f.sep), entries
}

func (f *format) Render(prefix string, entries List[string]) string {
	if entries == nil || entries.Len() == 0 {
		return prefix
	}
	var sb strings.Builder
	sb.WriteString(prefix)
	for i, e := range entries.Entries() {
		switch {
		case i > 0:
			sb.WriteString(f.sep)
		case prefix == "" || strings.HasSuffix(prefix, f.opener()):
			// The string is nothing but pairs, or already opens the section.
		default:
			sb.WriteString(f.opener())
		}
		if !e.Keyed {
			sb.WriteString(e.Raw)
			continue
		}
		fmt.Fprintf(&sb, "%s%s%s%s", f.keyPrefix, e.Key, f.equal, e.Value)
	}
	return sb.String()
}

func (f *format) Apply(s string, params map[string]string) string {
	prefix, entries := f.Parse(s)
	// Setting params last overwrites in place what s spelled out, and setting
	// them in sorted order fixes where the rest land.
	for _, k := range slices.Sorted(maps.Keys(params)) {
		entries.Set(k, params[k])
	}
	return f.Render(prefix, entries)
}

// parse reads every token as a pair, keeping the order they appear in. ok is
// false as soon as a token is not one.
func (f *format) parse(tokens []string) (List[string], bool) {
	entries := New[string]()
	for _, token := range tokens {
		k, v, ok := f.cut(token)
		if !ok {
			return entries, false
		}
		entries.Set(k, v)
	}
	return entries, true
}

// cut splits a token into the key and value of a pair. A key that is missing
// its prefix, or that holds a separator, an opener or a mark of a URI, means the
// token is text rather than a pair.
func (f *format) cut(token string) (string, string, bool) {
	if f.keyPrefix != "" {
		if !strings.HasPrefix(token, f.keyPrefix) {
			return "", "", false
		}
		token = token[len(f.keyPrefix):]
	}
	k, v, ok := strings.Cut(token, f.equal)
	if !ok || k == "" || strings.ContainsAny(k, badKeyChars) ||
		strings.Contains(k, f.sep) || (f.open != "" && strings.Contains(k, f.open)) {
		return "", "", false
	}
	return k, v, true
}
