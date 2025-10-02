package errors

import (
	"fmt"

	"k8s.io/apimachinery/pkg/labels"
)

type Error interface {
	Message() string // latest error message without cause
	Error() string   // all error messages concatenated
	Format(f fmt.State, c rune)
	Code() (string, labels.Set)
	Category() Category
	Has(cause error) bool
	Cause() error
	RootCause() error
	Chain() []error
}

type Category interface {
	Error() string
	StatusCode() int
	New(opts ...Option) error
	Newf(format string, args ...any) error
	Wrap(err error, opts ...Option) error
	Wrapf(err error, format string, args ...any) error
}
