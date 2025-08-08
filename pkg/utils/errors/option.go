package errors

import "fmt"

type Option func(*errBase)

func WithCode(code string, details map[string]string) Option {
	return func(b *errBase) {
		b.code = code
		b.details = details
	}
}

func WithFormat(format string, args ...any) Option {
	return func(b *errBase) {
		b.message = fmt.Sprintf(format, args...)
	}
}

func WithCategory(category Category) Option {
	return func(b *errBase) {
		b.category = category
	}
}
