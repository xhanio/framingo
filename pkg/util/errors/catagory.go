package errors

import (
	"net/http"
)

var _ Category = (*errCategory)(nil)

var (
	Cancaled = NewCategory("Cancelled", 499) // non-standard status code for cancellation

	BadRequest       = NewCategory("BadRequest", http.StatusBadRequest)
	InvalidArgument  = NewCategory("InvalidArgument", http.StatusBadRequest)
	Unauthorized     = NewCategory("Unauthorized", http.StatusUnauthorized)
	Forbidden        = NewCategory("Forbidden", http.StatusForbidden)
	PermissionDenied = NewCategory("PermissionDenied", http.StatusForbidden)
	NotFound         = NewCategory("NotFound", http.StatusNotFound)
	DeadlineExceeded = NewCategory("DeadlineExceeded", http.StatusRequestTimeout)
	Conflict         = NewCategory("Conflict", http.StatusConflict)
	AlreadyExist     = NewCategory("AlreadyExist", http.StatusConflict)
	TooManyRequests  = NewCategory("TooManyRequests", http.StatusTooManyRequests)

	Internal          = NewCategory("Internal", http.StatusInternalServerError)
	NotImplemented    = NewCategory("NotImplemented", http.StatusNotImplemented)
	Unavailable       = NewCategory("Unavailable", http.StatusServiceUnavailable)
	ResourceExhausted = NewCategory("ResourceExhausted", http.StatusServiceUnavailable)

	DBFailed = NewCategory("DBFailed", http.StatusInternalServerError)
)

type errCategory struct {
	description string
	status      int
}

func NewCategory(description string, status int) Category {
	return newCategory(description, status)
}

func newCategory(description string, status int) *errCategory {
	return &errCategory{description: description, status: status}
}

func (c *errCategory) Error() string {
	return c.description
}

func (c *errCategory) StatusCode() int {
	return c.status
}

func (c *errCategory) New(opts ...Option) error {
	if len(opts) == 0 {
		return c
	}
	opts = append(opts, WithCategory(c))
	return New(opts...)
}

func (c *errCategory) Newf(format string, args ...any) error {
	return c.New(WithFormat(format, args...))
}

func (c *errCategory) Wrap(err error, opts ...Option) error {
	opts = append(opts, WithCategory(c))
	return Wrap(err, opts...)
}

func (c *errCategory) Wrapf(err error, format string, args ...any) error {
	return c.Wrap(err, WithFormat(format, args...))
}
