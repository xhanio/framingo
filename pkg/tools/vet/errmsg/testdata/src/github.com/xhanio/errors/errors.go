package errors

type Option func()

type Category interface {
	Error() string
	New(opts ...Option) error
	Newf(format string, args ...any) error
	Wrap(err error, opts ...Option) error
	Wrapf(err error, format string, args ...any) error
}

var NotFound Category

func New(opts ...Option) error                          { return nil }
func Newf(format string, args ...any) error             { return nil }
func Wrap(err error, opts ...Option) error              { return nil }
func Wrapf(err error, format string, args ...any) error { return nil }
func WithMessage(format string, args ...any) Option     { return nil }
