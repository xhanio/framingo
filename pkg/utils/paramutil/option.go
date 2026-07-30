package paramutil

import "slices"

type ParamsOption func(*params)

// WithOpener sets the text that starts the section of pairs, such as the "?" of
// a URL query. Without it the whole string is a section.
func WithOpener(open string) ParamsOption {
	return func(p *params) {
		p.open = open
	}
}

// WithKeyPrefix sets the prefixes a key may carry, such as the "--" and "-" of
// command line flags. Text carrying none of them is not a pair, a key keeps the
// prefix it was written with, and a name given without one takes the first of
// these. The prefixes are taken as they are then, never aliased.
func WithKeyPrefix(prefixes ...string) ParamsOption {
	return func(p *params) {
		p.keyPrefixes = slices.Clone(prefixes)
	}
}

// WithQuote sets the quotes a value may be wrapped in, such as the "'" of a
// libpq connection string. A quote only counts where a value starts, so an
// apostrophe inside one is text like any other; the separator inside a quoted
// value does not end it; and rendering reaches for the first of these the value
// does not itself hold. Without quotes a value can be neither empty nor hold
// the separator and still read back.
func WithQuote(quotes ...string) ParamsOption {
	return func(p *params) {
		p.quotes = slices.Clone(quotes)
	}
}

// WithBadKeyChars sets the characters that keep a name from being one, which
// are "/@:" until a notation says otherwise: the marks of a URI or a credential,
// there to stop a connection string handed to the wrong notation from reading
// as a pair. Pass "" where a key may hold anything, as an environment variable
// name may.
func WithBadKeyChars(chars string) ParamsOption {
	return func(p *params) {
		p.badKeys = chars
	}
}

// AllowRaw lets a section hold text that is not a pair - a command, a
// positional, a flag that takes no value - keeping it in place rather than
// ending the section at it.
func AllowRaw() ParamsOption {
	return func(p *params) {
		p.raw = true
	}
}
