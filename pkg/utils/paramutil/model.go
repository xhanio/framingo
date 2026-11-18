// Package paramutil parses, renders and merges key=value params in several notations.
package paramutil

var (
	_ Entries[string] = (*entries[string])(nil)
	_ Params          = (*params)(nil)
	_ Args            = (*args)(nil)
)

// Param carries Key and Value when Keyed, Raw otherwise.
type Param[V any] struct {
	Key   string
	Value V
	Raw   string
	Keyed bool
}

// Entries is an ordered set of params: a key keeps its first position and its last value;
// raw entries are never merged. Not safe for concurrent use.
type Entries[V any] interface {
	// Set overwrites in place or appends; "" is a valid key.
	Set(key string, value V)
	// AddRaw appends unkeyed text, invisible to Get.
	AddRaw(raw string)
	Get(key string) (V, bool)
	// Len counts keyed and raw params alike.
	Len() int
	// List returns a fresh shallow copy, in order.
	List() []Param[V]
}

// Arg is a flag's values; Inline means they followed the equal sign rather than separate tokens.
type Arg struct {
	Inline bool
	Values []string
}

// Args is the command-line notation, where a flag may take values from the following tokens.
// A non-flag token is a value of the open flag, else a positional; a repeated flag keeps its
// first position and its values are replaced, not appended.
type Args interface {
	Parse(tokens []string) Entries[Arg]
	// ParseInto merges into existing entries (must not be nil), left to right; an open flag
	// does not carry over between calls.
	ParseInto(entries Entries[Arg], tokens []string)
	Render(entries Entries[Arg]) []string
}

// Params is a key=value&key=value notation (equal, separator, opener, key prefixes, quotes).
// Values are quoted only when they would not read back as themselves; there is no escape, so a
// value holding every quote cannot round-trip. A name (key minus prefix) must be non-empty and
// free of the opener and bad chars ("/@:" by default), or the token is not a pair: it stays in
// the prefix, or is kept raw under AllowRaw. --a and -a are distinct keys.
type Params interface {
	// Parse splits s into prefix and section; the joining opener/separator stays on the prefix
	// so Render round-trips. A string with no opener is read as a section ("a=b&c=d").
	Parse(s string) (string, Entries[string])
	// ParseTokens reads one pair per token; non-pairs are kept raw regardless of AllowRaw, and a
	// separator inside a token is plain text.
	ParseTokens(tokens []string) Entries[string]
	// ParseTokensInto merges into existing entries (must not be nil), left to right.
	ParseTokensInto(entries Entries[string], tokens []string)
	// Render appends the opener unless prefix is empty or already ends with opener or separator;
	// keys and raw entries are written unvalidated.
	Render(prefix string, entries Entries[string]) string
	// RenderTokens writes one token per entry, quoting only values a reader would unquote.
	RenderTokens(entries Entries[string]) []string
	// Apply overwrites existing keys in place and appends new ones in sorted order; a bare name
	// takes the notation's first key prefix.
	Apply(s string, params map[string]string) string
}
