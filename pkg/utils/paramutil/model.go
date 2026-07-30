// Package paramutil reads and writes parameters in the notations they are
// spelled in - a=b&c=d, a=b c='d e', a=b,c=d, --a=b -v - and merges one set of
// them over another, a key keeping the place it was first written at while
// taking the value it was last given.
package paramutil

var (
	_ Entries[string] = (*entries[string])(nil)
	_ Params          = (*params)(nil)
	_ Args            = (*args)(nil)
)

// Param is one element of an Entries: one parameter of whatever it holds. Key
// and Value carry it when Keyed is true; Raw carries it otherwise.
type Param[V any] struct {
	Key   string
	Value V
	Raw   string
	Keyed bool
}

// Entries is an ordered sequence of params. A keyed one keeps the position it
// was first set at and takes the value of the last Set for that key. An unkeyed
// one holds text that passes through in the position it was added and is never
// merged with anything.
//
// Entries is not safe for concurrent use.
type Entries[V any] interface {
	// Set records value under key, overwriting the param in place when the key
	// is already there and appending it otherwise. The empty string is a key
	// like any other.
	Set(key string, value V)
	// AddRaw appends text that carries no key. It never merges with another
	// param and never answers a Get.
	AddRaw(raw string)
	// Get returns the value recorded under key.
	Get(key string) (V, bool)
	// Len returns the number of params, keyed and unkeyed alike.
	Len() int
	// List returns the params in order. The result is newly allocated and never
	// aliases the Entries, though a stored pointer or slice still points where
	// it did.
	List() []Param[V]
}

// Arg is what a flag holds: the values it was given, and whether they were
// written after the equal sign rather than in the tokens that followed it.
type Arg struct {
	Inline bool
	Values []string
}

// Args is the notation of a command line, where a flag may take its value from
// the tokens that follow it rather than from its own. That is the one thing
// Params cannot describe: a value spanning tokens is a list of them, and Params
// reads one value from one token.
//
// A token naming no flag is a positional where no flag stands open, and a value
// of that flag where one does. A flag seen again keeps the position it was
// first seen at and starts its values over, so a later line replaces an earlier
// one rather than adding to it.
type Args interface {
	// Parse reads tokens as a command line.
	Parse(tokens []string) Entries[Arg]
	// ParseInto reads tokens into an Entries that may already hold params, so
	// lines read one after another merge left to right. A flag left open at the
	// end of one call does not take the tokens of the next. The entries must
	// not be nil.
	ParseInto(entries Entries[Arg], tokens []string)
	// Render writes entries back as a command line, a flag and the values it
	// did not take inline standing as tokens of their own.
	Render(entries Entries[Arg]) []string
}

// Params is the notation of a string that spells a list of params out as
// key1=value1&key2=value2. Five things describe one: the equal sign joining a
// key to its value, the separator joining one pair to the next, the opener that
// starts the section when it follows text that is not a pair, such as a URL
// path or a credential, the prefixes a key may carry, such as the "--" and "-"
// of command line flags, and the quotes a value may be wrapped in.
//
// A value is quoted where it would not otherwise read back as itself: where it
// is empty, where it holds the separator, where it begins with a quote, or
// where a reader would unquote it. A notation with no quotes has no way to
// write those values, and one with quotes has no way to write a value that
// holds every quote it knows - or begins with one and holds the rest - there
// being no escape.
//
// A name - what is left of a key once its prefix is off - may not be empty or
// hold the opener or any character the notation calls bad, which is "/@:" until
// WithBadKeyChars says otherwise; and where a notation asks for prefixes a key
// must carry one. Text that reads otherwise is not a pair: it stays in front of
// the section, or is kept where it stands if the notation AllowRaw. That is
// what keeps a "?" inside a password from opening a query, and what lets
// "mycmd --a=b -v" keep its command and its bare flag.
//
// A key keeps the prefix it was written with, so --a and -a are two keys.
type Params interface {
	// Parse splits s into the text preceding its section and the params of
	// that section, in the order they appear. Whatever joined the two - the
	// opener, or the separator where the section merely follows text - stays on
	// the prefix, so Render rejoins them with what actually stood there. A key
	// the section repeats is kept once and takes its last value. A string that
	// never opens a section is read as one, so "a=b&c=d" parses without a "?"
	// in front of it.
	Parse(s string) (string, Entries[string])
	// ParseTokens reads each token as a pair, in the order they appear, for a
	// caller whose separator is the boundary between tokens rather than text of
	// its own. A token that is not a pair is kept as a raw entry whether or not
	// the notation AllowRaw, there being no text in front of a section here for
	// it to belong to instead. A separator inside a token is text - in the name
	// and the value alike - no quotes needed, nothing having been split on it.
	ParseTokens(tokens []string) Entries[string]
	// ParseTokensInto reads tokens into an Entries that may already hold params,
	// as ParseTokens does otherwise. A key it already holds keeps its
	// position and takes the new value, so sets read one after another merge
	// left to right. The entries must not be nil.
	ParseTokensInto(entries Entries[string], tokens []string)
	// Render writes entries as a section of prefix, opening it unless prefix is
	// empty or already ends with the opener or the separator. Raw entries and
	// keys are written as they are, valid in the notation or not.
	Render(prefix string, entries Entries[string]) string
	// RenderTokens writes entries as one token each, raw entries as they are.
	// A value needs no quotes here for being empty or holding the separator,
	// the token boundary having done that work; only a value a reader would
	// unquote is quoted, so that it reads back as itself.
	RenderTokens(entries Entries[string]) []string
	// Apply merges params into the pairs already in s. A key s carries keeps its
	// position and takes its value from params; the rest are appended in sorted
	// order, so the result is deterministic. A param named without a prefix
	// takes the notation's first one, so params stay free of the notation they
	// are applied to.
	Apply(s string, params map[string]string) string
}
