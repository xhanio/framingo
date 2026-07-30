package paramutil

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// badKeyChars are the characters that tell a name apart from a fragment of a
// URI or a credential, and are what a notation rejects in one unless
// WithBadKeyChars says otherwise.
const badKeyChars = "/@:"

// The notations a set of params is commonly spelled in.
var (
	// Query reads a=b&c=d, opened by the "?" of a URL when one precedes it.
	// Values go unquoted, the web having percent-encoding for what quotes are
	// for elsewhere.
	Query = NewParams("=", "&", WithOpener("?"))
	// Keyword reads a=b c='d e', as a libpq connection string does.
	Keyword = NewParams("=", " ", WithQuote("'"))
	// Flags reads --a=b -c="d e", as command line flags do, and keeps the words
	// among them: the command, a positional, a flag that takes no value.
	Flags = NewParams("=", " ", WithKeyPrefix("--", "-"), WithQuote(`"`, "'"), AllowRaw())
	// Comma reads a=b,c="d,e".
	Comma = NewParams("=", ",", WithQuote(`"`, "'"))
	// Env reads KEY=VALUE entries as a process environment holds them: any
	// name at all, one entry per token, joined by the NUL that separates an
	// environment block. Quotes are text - the shell's business, not the
	// environment's - and an entry with nothing before its equal sign is not
	// a pair, which keeps the =C: entries of a Windows environment intact.
	Env = NewParams("=", "\x00", WithBadKeyChars(""))
)

type params struct {
	equal       string
	sep         string
	open        string
	keyPrefixes []string
	quotes      []string
	badKeys     string
	raw         bool
}

// NewParams returns the notation whose pairs read as key<equal>value and are
// joined by sep. Without WithOpener the whole string is a section, and sep
// divides it from any text that precedes it.
func NewParams(equal, sep string, opts ...ParamsOption) Params {
	p := &params{
		equal:   equal,
		sep:     sep,
		badKeys: badKeyChars,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// opener returns the text that starts the section. A notation with no opener of
// its own reuses its separator, having nothing else to divide a prefix from the
// pairs that follow it.
func (p *params) opener() string {
	if p.open == "" {
		return p.sep
	}
	return p.open
}

func (p *params) Parse(s string) (string, Entries[string]) {
	if p.open != "" {
		if i := strings.LastIndex(s, p.open); i >= 0 {
			prefix, tail := s[:i+len(p.open)], s[i+len(p.open):]
			if tail == "" {
				return prefix, NewEntries[string]()
			}
			entries := NewEntries[string]()
			if p.read(entries, p.tokens(tail), p.raw) {
				return prefix, entries
			}
		}
		// Either nothing opened a section, or what followed the opener was not
		// one. The string may still be pairs and nothing else, but read it
		// strictly: with no opener to mark where the section starts, text that
		// is not a pair belongs in front of it rather than in it.
		return p.split(s, false)
	}
	return p.split(s, p.raw)
}

func (p *params) ParseTokens(tokens []string) Entries[string] {
	entries := NewEntries[string]()
	p.ParseTokensInto(entries, tokens)
	return entries
}

func (p *params) ParseTokensInto(entries Entries[string], tokens []string) {
	// Raw whatever the notation says: with the tokens already divided there is
	// no prefix for one that is not a pair to fall into.
	p.read(entries, tokens, true)
}

func (p *params) RenderTokens(entries Entries[string]) []string {
	if entries == nil {
		return nil
	}
	out := make([]string, 0, entries.Len())
	for _, e := range entries.List() {
		if !e.Keyed {
			out = append(out, e.Raw)
			continue
		}
		out = append(out, e.Key+p.equal+p.quoteToken(e.Value))
	}
	return out
}

func (p *params) Render(prefix string, entries Entries[string]) string {
	if entries == nil || entries.Len() == 0 {
		return prefix
	}
	var sb strings.Builder
	sb.WriteString(prefix)
	for i, e := range entries.List() {
		switch {
		case i > 0:
			sb.WriteString(p.sep)
		case prefix == "" || strings.HasSuffix(prefix, p.opener()) || strings.HasSuffix(prefix, p.sep):
			// The string is nothing but a section, already opens one, or ends
			// with the separator that joined it to one.
		default:
			sb.WriteString(p.opener())
		}
		if !e.Keyed {
			sb.WriteString(e.Raw)
			continue
		}
		fmt.Fprintf(&sb, "%s%s%s", e.Key, p.equal, p.quote(e.Value))
	}
	return sb.String()
}

func (p *params) Apply(s string, params map[string]string) string {
	prefix, entries := p.Parse(s)
	// Setting params last overwrites in place what s spelled out, and setting
	// them in sorted order fixes where the rest land.
	for _, k := range slices.Sorted(maps.Keys(params)) {
		entries.Set(p.key(k), params[k])
	}
	return p.Render(prefix, entries)
}

// split divides s into the text in front of its section and the section itself,
// scanning back from the end for as long as the tokens belong to it. A section
// that holds raw entries starts at the beginning, there being no token it
// cannot hold.
func (p *params) split(s string, raw bool) (string, Entries[string]) {
	entries := NewEntries[string]()
	if s == "" {
		return "", entries
	}
	tokens := p.tokens(s)
	i := 0
	if !raw {
		for i = len(tokens); i > 0; i-- {
			if _, _, ok := p.cut(tokens[i-1]); !ok {
				break
			}
		}
	}
	p.read(entries, tokens[i:], raw)
	prefix := strings.Join(tokens[:i], p.sep)
	if 0 < i && i < len(tokens) {
		// The separator that joined the prefix to its section stays on the
		// prefix, as the opener does when one opened it, so rendering rejoins
		// them with what actually stood there.
		prefix += p.sep
	}
	return prefix, entries
}

// read takes every token as a pair into entries, keeping the order they appear
// in. A token that is not one is kept as a raw entry where the section holds
// them, and otherwise fails the read.
func (p *params) read(entries Entries[string], tokens []string, raw bool) bool {
	for _, token := range tokens {
		k, v, ok := p.cut(token)
		if !ok {
			if !raw {
				return false
			}
			entries.AddRaw(token)
			continue
		}
		entries.Set(k, v)
	}
	return true
}

// cut splits a token into the key and value of a pair, the key keeping whatever
// prefix it carries. A token that is missing a prefix the notation asks for, or
// whose name holds an opener or a mark of a URI, is text rather than a pair. No
// rule bars a name from the separator: a token cut out of a string cannot hold
// one, the string having been split on it, and in a token list the separator is
// text like any other.
func (p *params) cut(token string) (string, string, bool) {
	k, v, ok := strings.Cut(token, p.equal)
	if !ok {
		return "", "", false
	}
	name := k
	if len(p.keyPrefixes) > 0 {
		prefix, ok := p.prefixOf(k)
		if !ok {
			return "", "", false
		}
		name = k[len(prefix):]
	}
	if name == "" || strings.ContainsAny(name, p.badKeys) ||
		(p.open != "" && strings.Contains(name, p.open)) {
		return "", "", false
	}
	return k, p.unquote(v), true
}

// tokens divides s at every separator that stands outside a quoted value. A
// quote counts only where a value starts, right after the equal sign, so an
// apostrophe inside a value is text like any other. An unclosed quote runs to
// the end of s and leaves the value as it was written.
func (p *params) tokens(s string) []string {
	if len(p.quotes) == 0 {
		return strings.Split(s, p.sep)
	}
	var out []string
	start, quote := 0, ""
	for i := 0; i < len(s); {
		if quote != "" {
			if strings.HasPrefix(s[i:], quote) {
				i += len(quote)
				quote = ""
				continue
			}
			i++
			continue
		}
		if q, ok := p.quoteAt(s, start, i); ok {
			i += len(q)
			quote = q
			continue
		}
		if strings.HasPrefix(s[i:], p.sep) {
			out = append(out, s[start:i])
			i += len(p.sep)
			start = i
			continue
		}
		i++
	}
	return append(out, s[start:])
}

// quoteAt returns the quote opening a value at i: one the notation knows,
// standing right after the first equal sign of the token that began at start -
// where a value starts, and nowhere else.
func (p *params) quoteAt(s string, start, i int) (string, bool) {
	seg := s[start:i]
	if !strings.HasSuffix(seg, p.equal) || strings.Index(seg, p.equal) != len(seg)-len(p.equal) {
		return "", false
	}
	for _, q := range p.quotes {
		if strings.HasPrefix(s[i:], q) {
			return q, true
		}
	}
	return "", false
}

// unquote takes off the quote a value is wrapped in.
func (p *params) unquote(v string) string {
	for _, q := range p.quotes {
		if len(v) >= 2*len(q) && strings.HasPrefix(v, q) && strings.HasSuffix(v, q) {
			return v[len(q) : len(v)-len(q)]
		}
	}
	return v
}

// quote wraps a value that would not read back as itself once a string is split
// on its separator: one that is empty, one holding the separator, or one
// beginning with a quote, which would put the reader into one.
func (p *params) quote(v string) string {
	if v == "" || strings.Contains(v, p.sep) || p.opensQuote(v) {
		return p.wrap(v)
	}
	return p.quoteToken(v)
}

// opensQuote reports whether a bare v begins with one of the notation's quotes,
// the one place a reader takes a quote for the notation's own.
func (p *params) opensQuote(v string) bool {
	for _, q := range p.quotes {
		if strings.HasPrefix(v, q) {
			return true
		}
	}
	return false
}

// quoteToken wraps a value a reader would unquote, which is all a value written
// as a token of its own needs: nothing was split on the separator, so nothing
// has to hold the value together across it.
func (p *params) quoteToken(v string) string {
	if p.unquote(v) == v {
		return v
	}
	return p.wrap(v)
}

// wrap puts the first quote the value does not itself hold around it. Where the
// value holds every quote the notation knows - there being no escape for one -
// it is written as it is.
func (p *params) wrap(v string) string {
	for _, q := range p.quotes {
		if !strings.Contains(v, q) {
			return q + v + q
		}
	}
	return v
}

// key returns the key a name is set under. A name already carrying one of the
// notation's prefixes is left as it is, and a bare one takes the first prefix,
// so params written without prefixes still land on the keys they name.
func (p *params) key(name string) string {
	if len(p.keyPrefixes) == 0 {
		return name
	}
	if _, ok := p.prefixOf(name); ok {
		return name
	}
	return p.keyPrefixes[0] + name
}

// prefixOf returns the longest of the notation's prefixes that k carries.
func (p *params) prefixOf(k string) (string, bool) {
	var longest string
	found := false
	for _, prefix := range p.keyPrefixes {
		if strings.HasPrefix(k, prefix) && (!found || len(prefix) > len(longest)) {
			longest, found = prefix, true
		}
	}
	return longest, found
}
