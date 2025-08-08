package errors

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/labels"
)

var _ Base = (*errBase)(nil)

type errBase struct {
	stack *stack

	category Category
	code     string
	message  string
	details  labels.Set
	cause    error
}

func (b *errBase) apply(opts ...Option) *errBase {
	for _, opt := range opts {
		opt(b)
	}
	return b
}

func (b *errBase) rootBase() *errBase {
	curr := b
	for curr != nil && curr.cause != nil {
		cb, ok := curr.cause.(*errBase)
		if !ok {
			return curr
		}
		curr = cb
	}
	return curr
}

func (b *errBase) Message() string {
	curr := b
	for curr != nil && curr.message == "" {
		cb, ok := curr.cause.(*errBase)
		if !ok {
			return curr.cause.Error()
		}
		curr = cb
	}
	return curr.message
}

func (b *errBase) Error() string {
	var builder strings.Builder
	curr := b
	for curr != nil {
		if curr.message != "" {
			if builder.Len() > 0 {
				builder.WriteString(": ")
			}
			builder.WriteString(curr.message)
		}
		cb, ok := curr.cause.(*errBase)
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

func (b *errBase) Code() (string, labels.Set) {
	curr := b
	for curr != nil {
		if curr.code != "" {
			return curr.code, curr.details
		}
		cb, ok := curr.cause.(*errBase)
		if !ok {
			break
		}
		curr = cb
	}
	return "", nil
}

func (b *errBase) Category() Category {
	curr := b
	for curr != nil {
		if curr.category != nil {
			return curr.category
		}
		cc, ok := curr.cause.(*errCategory)
		if ok {
			return cc
		}
		cb, ok := curr.cause.(*errBase)
		if !ok {
			break
		}
		curr = cb
	}
	return Internal
}

func (b *errBase) Cause() error {
	return b.cause
}

func (b *errBase) StackTrace() errors.StackTrace {
	rb := b.rootBase()
	if rb == nil || rb.stack == nil {
		return nil
	}
	return rb.stack.StackTrace()
}

func (b *errBase) Format(f fmt.State, c rune) {
	msg := b.Error()
	switch c {
	case 'm':
		f.Write([]byte(b.Message()))
	case 's':
		f.Write([]byte(msg))
	case 'v':
		code, details := b.Code()
		if code != "" {
			msg = fmt.Sprintf("{%s} %s", strings.Join([]string{code, details.String()}, ":"), msg)
		}
		f.Write([]byte(fmt.Sprintf("%s%+v",
			msg,
			b.StackTrace(),
		)))
	default:
		f.Write([]byte(fmt.Sprintf("!%%%c(%s)", c, msg)))
	}
}

func (b *errBase) Has(cause error) bool {
	curr := b
	for curr != nil && curr.cause != nil {
		if curr == cause {
			return true
		}
		cb, ok := curr.cause.(*errBase)
		if !ok {
			return curr.cause == cause
		}
		curr = cb
	}
	return curr == cause
}

func (b *errBase) RootCause() error {
	rb := b.rootBase()
	if rb.cause != nil {
		return rb.cause
	}
	return rb
}
