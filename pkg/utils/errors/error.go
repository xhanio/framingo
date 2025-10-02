package errors

import (
	"go.uber.org/multierr"
)

func Combine(errors ...error) error {
	return multierr.Combine(errors...)
}

func New(opts ...Option) error {
	b := &base{
		stack: callers(),
	}
	return b.apply(opts...)
}

func Newf(format string, args ...any) error {
	b := &base{
		stack: callers(),
	}
	return b.apply(WithMessage(format, args...))
}

func Wrap(err error, opts ...Option) error {
	if err == nil {
		return nil
	}
	_, ok := err.(*base)
	if ok && len(opts) == 0 {
		return err
	}
	b := &base{
		cause: err,
	}
	if !ok {
		b.stack = callers()
	}
	return b.apply(opts...)
}

func Wrapf(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	_, ok := err.(*base)
	if ok && format == "" {
		return err
	}
	b := &base{
		cause: err,
	}
	if !ok {
		b.stack = callers()
	}
	return b.apply(WithMessage(format, args...))
}

func Has(err error, cause error) bool {
	be, ok := err.(Error)
	if !ok {
		return be == cause
	}
	return be.Has(cause)
}

func Is(err error, c Category) bool {
	be, ok := err.(Error)
	if !ok {
		return false
	}
	return be.Category() == c
}

func Message(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}
