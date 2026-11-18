package paramutil

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// badKeyChars mark a URI or credential fragment rather than a name.
const badKeyChars = "/@:"

var (
	// Query: a=b&c=d after an optional "?"; unquoted, as the web percent-encodes instead.
	Query = NewParams("=", "&", WithOpener("?"))
	// Keyword: a=b c='d e' (libpq connection string).
	Keyword = NewParams("=", " ", WithQuote("'"))
	// Flags: --a=b -c="d e", keeping bare words (command, positionals, valueless flags) raw.
	Flags = NewParams("=", " ", WithKeyPrefix("--", "-"), WithQuote(`"`, "'"), AllowRaw())
	Comma = NewParams("=", ",", WithQuote(`"`, "'"))
	// Env: NUL-joined KEY=VALUE, any name, quotes are text; "=C:"-style Windows entries stay raw.
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

// NewParams: without WithOpener, sep also divides the section from any preceding text.
func NewParams(equal, sep string, opts ...ParamsOption) Params {
	p := &params{
		equal:   equal,
		sep:     sep,
		badKeys: badKeyChars,
	}
	p.applyOptions(opts...)
	return p
}

func (p *params) applyOptions(opts ...ParamsOption) {
	for _, opt := range opts {
		opt(p)
	}
}

// opener falls back to the separator when the notation has none.
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
		// no usable opener: read strictly, so non-pair text goes to the prefix
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
	// always raw: pre-split tokens have no prefix for non-pairs to fall into
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
			// already opened: write no opener
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
	// sorted so appended keys land deterministically
	for _, k := range slices.Sorted(maps.Keys(params)) {
		entries.Set(p.key(k), params[k])
	}
	return p.Render(prefix, entries)
}

// split scans back from the end while tokens are pairs; a raw section takes everything.
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
		// keep the joining separator on the prefix so Render round-trips
		prefix += p.sep
	}
	return prefix, entries
}

// read fails on a non-pair token unless raw.
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

// cut keeps the key's prefix. The separator is deliberately allowed in a name: split strings
// cannot contain it, and in token lists it is plain text.
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

// tokens splits on separators outside quotes; a quote opens only right after the equal sign,
// and an unclosed one runs to the end.
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

func (p *params) unquote(v string) string {
	for _, q := range p.quotes {
		if len(v) >= 2*len(q) && strings.HasPrefix(v, q) && strings.HasSuffix(v, q) {
			return v[len(q) : len(v)-len(q)]
		}
	}
	return v
}

func (p *params) quote(v string) string {
	if v == "" || strings.Contains(v, p.sep) || p.opensQuote(v) {
		return p.wrap(v)
	}
	return p.quoteToken(v)
}

func (p *params) opensQuote(v string) bool {
	for _, q := range p.quotes {
		if strings.HasPrefix(v, q) {
			return true
		}
	}
	return false
}

func (p *params) quoteToken(v string) string {
	if p.unquote(v) == v {
		return v
	}
	return p.wrap(v)
}

// wrap uses the first quote v lacks; with none left (no escape exists) v is written bare.
func (p *params) wrap(v string) string {
	for _, q := range p.quotes {
		if !strings.Contains(v, q) {
			return q + v + q
		}
	}
	return v
}

func (p *params) key(name string) string {
	if len(p.keyPrefixes) == 0 {
		return name
	}
	if _, ok := p.prefixOf(name); ok {
		return name
	}
	return p.keyPrefixes[0] + name
}

// prefixOf returns the longest matching prefix.
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
