package kv

type FormatOption func(*format)

// WithOpener sets the text that starts the section of pairs, such as the "?" of
// a URL query. Without it every token of the string is read as a pair.
func WithOpener(open string) FormatOption {
	return func(f *format) {
		f.open = open
	}
}

// WithKeyPrefix sets the text every key carries, such as the "--" of a long
// command line flag. Text that does not carry it is not a pair.
func WithKeyPrefix(prefix string) FormatOption {
	return func(f *format) {
		f.keyPrefix = prefix
	}
}
