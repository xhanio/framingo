package errors

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/labels"
)

var _ Error = (*base)(nil)

type base struct {
	stack *stack

	category Category
	code     string
	message  string
	details  labels.Set
	cause    error
}

// apply executes the given options on the error base and returns the modified error base
func (b *base) apply(opts ...Option) *base {
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// root traverses the error chain to find the root error base
func (b *base) rootBase() *base {
	curr := b
	for curr != nil && curr.cause != nil {
		cb, ok := curr.cause.(*base)
		if !ok {
			return curr
		}
		curr = cb
	}
	return curr
}

// Message returns the error message, traversing the cause chain if the current message is empty
func (b *base) Message() string {
	curr := b
	for curr != nil && curr.message == "" {
		cb, ok := curr.cause.(*base)
		if !ok {
			return curr.cause.Error()
		}
		curr = cb
	}
	return curr.message
}

// Error returns the formatted error string by concatenating all messages in the error chain
func (b *base) Error() string {
	var builder strings.Builder
	curr := b
	for curr != nil {
		if curr.message != "" {
			if builder.Len() > 0 {
				builder.WriteString(": ")
			}
			builder.WriteString(curr.message)
		}
		cb, ok := curr.cause.(*base)
		if !ok {
			if curr.cause != nil {
				if builder.Len() > 0 {
					builder.WriteString(": ")
				}
				builder.WriteString(curr.cause.Error())
			}
			break
		}
		curr = cb
	}
	return builder.String()
}

// Code returns the error code and details by traversing the cause chain until a non-empty code is found
func (b *base) Code() (string, labels.Set) {
	curr := b
	for curr != nil {
		if curr.code != "" {
			return curr.code, curr.details
		}
		cb, ok := curr.cause.(*base)
		if !ok {
			break
		}
		curr = cb
	}
	return "", nil
}

// Category returns the error category by traversing the cause chain, defaulting to Internal if none found
func (b *base) Category() Category {
	curr := b
	for curr != nil {
		if curr.category != nil {
			return curr.category
		}
		cc, ok := curr.cause.(*errCategory)
		if ok {
			return cc
		}
		cb, ok := curr.cause.(*base)
		if !ok {
			break
		}
		curr = cb
	}
	return Internal
}

// Cause returns the underlying cause of this error
func (b *base) Cause() error {
	return b.cause
}

// StackTrace returns the stack trace from the root error in the chain
func (b *base) StackTrace() errors.StackTrace {
	rb := b.rootBase()
	if rb == nil || rb.stack == nil {
		return nil
	}
	return rb.stack.StackTrace()
}

// Format implements fmt.Formatter interface to provide custom error formatting
func (b *base) Format(f fmt.State, c rune) {
	msg := b.Error()
	switch c {
	case 'm':
		fmt.Fprint(f, b.Message())
	case 's':
		fmt.Fprint(f, msg)
	case 'v':
		code, details := b.Code()
		if code != "" {
			msg = fmt.Sprintf("{%s}%s", strings.Join([]string{code, details.String()}, ":"), msg)
		}
		fmt.Fprintf(f, "%s%+v", msg, b.StackTrace())
	default:
		fmt.Fprintf(f, "!%%%c(%s)", c, msg)
	}
}

// Has checks if the given error exists anywhere in the error chain
func (b *base) Has(cause error) bool {
	curr := b
	for curr != nil && curr.cause != nil {
		if curr == cause {
			return true
		}
		cb, ok := curr.cause.(*base)
		if !ok {
			return curr.cause == cause
		}
		curr = cb
	}
	return curr == cause
}

// RootCause returns the root cause error at the end of the error chain
func (b *base) RootCause() error {
	r := b.rootBase()
	if r.cause != nil {
		return r.cause
	}
	return r
}

// Chain returns a slice of all errors in the error chain, starting from the current error
func (b *base) Chain() []error {
	var errs []error
	curr := b
	for curr != nil {
		errs = append(errs, curr)
		if curr.cause == nil {
			break
		}
		cb, ok := curr.cause.(*base)
		if !ok {
			break
		}
		curr = cb
	}
	return errs
}
