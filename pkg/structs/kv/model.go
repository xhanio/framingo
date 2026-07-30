package kv

var (
	_ List[string] = (*list[string])(nil)
	_ Format       = (*format)(nil)
)

// Entry is one element of a List. Key and Value carry the entry when Keyed is
// true; Raw carries it otherwise.
type Entry[V any] struct {
	Key   string
	Value V
	Raw   string
	Keyed bool
}

// List is an ordered sequence of entries. A keyed entry keeps the position it
// was first set at and takes the value of the last Set for that key. An unkeyed
// entry holds text that passes through in the position it was added and is
// never merged with anything.
//
// A List is not safe for concurrent use.
type List[V any] interface {
	// Set records value under key, overwriting the entry in place when the key
	// is already in the list and appending it otherwise. The empty string is a
	// key like any other.
	Set(key string, value V)
	// AddRaw appends text that carries no key. It never merges with another
	// entry and never answers a Get.
	AddRaw(raw string)
	// Get returns the value recorded under key.
	Get(key string) (V, bool)
	// Len returns the number of entries, keyed and unkeyed alike.
	Len() int
	// Entries returns the entries in order. The result is newly allocated and
	// never aliases the List, though a stored pointer still points where it did.
	Entries() []Entry[V]
}

// Format is the notation of a string that spells a list out as
// key1=value1&key2=value2: an equal sign joining a key to its value, a
// separator joining one pair to the next, and an opener that starts the section
// when it follows text that is not a pair, such as a URL path or a credential.
//
// A key may not hold the equal sign, the separator, the opener, or any of "/@:";
// text that reads otherwise is not a pair and stays out of the section. That is
// what keeps a "?" inside a password from opening a query.
type Format interface {
	// Parse splits s into the text preceding its section and the pairs of that
	// section, in the order they appear. A key the section repeats is kept once
	// and takes its last value. A string that never opens a section is read as
	// pairs and nothing else, so "a=b&c=d" parses without a "?" in front of it.
	Parse(s string) (string, List[string])
	// Render writes entries as a section of prefix, opening it unless prefix is
	// empty or already ends with the opener. Unkeyed entries are written as they
	// are.
	Render(prefix string, entries List[string]) string
	// Apply merges params into the pairs already in s. A key s carries keeps its
	// position and takes its value from params; the rest are appended in sorted
	// order, so the result is deterministic.
	Apply(s string, params map[string]string) string
}
