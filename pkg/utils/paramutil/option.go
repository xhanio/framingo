package paramutil

import "slices"

type ParamsOption func(*params)

// WithOpener sets the section opener (e.g. "?"); without it the whole string is a section.
func WithOpener(open string) ParamsOption {
	return func(p *params) {
		p.open = open
	}
}

// WithKeyPrefix makes a prefix mandatory for a pair; a bare name given to Apply takes the first.
func WithKeyPrefix(prefixes ...string) ParamsOption {
	return func(p *params) {
		p.keyPrefixes = slices.Clone(prefixes)
	}
}

// WithQuote sets value quotes, recognized only at a value's start; without quotes an empty
// value or one holding the separator cannot round-trip.
func WithQuote(quotes ...string) ParamsOption {
	return func(p *params) {
		p.quotes = slices.Clone(quotes)
	}
}

// WithBadKeyChars replaces the default "/@:" (which stops URIs reading as pairs); "" allows any name.
func WithBadKeyChars(chars string) ParamsOption {
	return func(p *params) {
		p.badKeys = chars
	}
}

// AllowRaw keeps non-pair text in place instead of ending the section at it.
func AllowRaw() ParamsOption {
	return func(p *params) {
		p.raw = true
	}
}
