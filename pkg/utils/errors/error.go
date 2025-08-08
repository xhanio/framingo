package errors

import (
	"go.uber.org/multierr"
)

func Combine(errors ...error) error {
	return multierr.Combine(errors...)
}

func New(opts ...Option) error {
	b := &errBase{
		stack: callers(),
	}
	return b.apply(opts...)
}

func Newf(format string, args ...any) error {
	b := &errBase{
		stack: callers(),
	}
	return b.apply(WithFormat(format, args...))
}

func Wrap(err error, opts ...Option) error {
	if err == nil {
		return nil
	}
	_, ok := err.(*errBase)
	if ok && len(opts) == 0 {
		return err
	}
	b := &errBase{
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
	_, ok := err.(*errBase)
	if ok && format == "" {
		return err
	}
	b := &errBase{
		cause: err,
	}
	if !ok {
		b.stack = callers()
	}
	return b.apply(WithFormat(format, args...))
}

func Has(err error, cause error) bool {
	be, ok := err.(Base)
	if !ok {
		return be == cause
	}
	return be.Has(cause)
}

func Is(err error, c Category) bool {
	be, ok := err.(Base)
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
