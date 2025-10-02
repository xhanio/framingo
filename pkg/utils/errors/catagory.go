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
	statusCode  int
}

// NewCategory creates a new error category with the given description and HTTP status code
func NewCategory(description string, statusCode int) Category {
	return newCategory(description, statusCode)
}

// newCategory creates a new errCategory instance with the given description and status code
func newCategory(description string, statusCode int) *errCategory {
	return &errCategory{description: description, statusCode: statusCode}
}

// Error returns the description of the error category, implementing the error interface
func (c *errCategory) Error() string {
	return c.description
}

// StatusCode returns the HTTP status code associated with this error category
func (c *errCategory) StatusCode() int {
	return c.statusCode
}

// New creates a new error with this category and the given options
func (c *errCategory) New(opts ...Option) error {
	if len(opts) == 0 {
		return c
	}
	opts = append(opts, WithCategory(c))
	return New(opts...)
}

// Newf creates a new error with this category and a formatted message
func (c *errCategory) Newf(format string, args ...any) error {
	return c.New(WithMessage(format, args...))
}

// Wrap wraps an existing error with this category and the given options
func (c *errCategory) Wrap(err error, opts ...Option) error {
	opts = append(opts, WithCategory(c))
	return Wrap(err, opts...)
}

// Wrapf wraps an existing error with this category and a formatted message
func (c *errCategory) Wrapf(err error, format string, args ...any) error {
	return c.Wrap(err, WithMessage(format, args...))
}
