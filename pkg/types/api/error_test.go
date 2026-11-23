package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/xhanio/labels"

	"github.com/xhanio/errors"
)

// The body, and the status metrics and spans record, come from the category the
// error resolves to, however it was wrapped on the way out of the handler.
func TestWrapErrorResolvesTheCategory(t *testing.T) {
	coded := errors.TooManyRequests.New(
		errors.WithMessage("slow down"),
		errors.WithCode("RATE_LIMIT", labels.Set{"ip": "10.0.0.1"}),
	)
	tests := []struct {
		name    string
		err     error
		status  int
		kind    string
		code    string
		details labels.Set
		message string
	}{
		{
			name:   "categorized",
			err:    errors.NotFound.Newf("user %s not found", "u1"),
			status: http.StatusNotFound, kind: "NotFound", message: "user u1 not found",
		},
		{
			name:   "only the latest level of the chain is exposed",
			err:    errors.Wrapf(errors.NotFound.Newf("row missing"), "user %s not found", "u1"),
			status: http.StatusNotFound, kind: "NotFound", message: "user u1 not found",
		},
		{
			name:   "wrapped by another package",
			err:    fmt.Errorf("handler: %w", errors.NotFound.Newf("gone")),
			status: http.StatusNotFound, kind: "NotFound", message: "gone",
		},
		{
			name:   "code and details",
			err:    errors.Wrap(coded),
			status: http.StatusTooManyRequests, kind: "TooManyRequests",
			code: "RATE_LIMIT", details: labels.Set{"ip": "10.0.0.1"}, message: "slow down",
		},
		{
			name:   "bare category",
			err:    errors.Unauthorized,
			status: http.StatusUnauthorized, kind: "Unauthorized",
		},
		{
			name:   "bare context.Canceled",
			err:    context.Canceled,
			status: 499, kind: "Canceled", message: "context canceled",
		},
		{
			name:   "wrapped context.DeadlineExceeded",
			err:    fmt.Errorf("query: %w", context.DeadlineExceeded),
			status: http.StatusGatewayTimeout, kind: "DeadlineExceeded", message: "Gateway Timeout",
		},
		{
			name:   "uncategorized",
			err:    fmt.Errorf("boom"),
			status: http.StatusInternalServerError, kind: "Internal", message: "Internal Server Error",
		},
		{
			name:   "undeclared combination",
			err:    errors.Combine(errors.BadRequest.Newf("name"), errors.BadRequest.Newf("age")),
			status: http.StatusInternalServerError, kind: "Combined", message: "Internal Server Error",
		},
		{
			name:   "declared combination",
			err:    errors.BadRequest.Wrapf(errors.Combine(errors.Newf("name"), errors.Newf("age")), "invalid request"),
			status: http.StatusBadRequest, kind: "BadRequest", message: "invalid request",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := WrapError(tt.err)
			assert.Same(t, tt.err, body.Origin)
			assert.Equal(t, tt.status, body.Status)
			assert.Equal(t, tt.kind, body.Kind)
			assert.Equal(t, tt.code, body.Code)
			assert.Equal(t, tt.details, body.Details)
			assert.Equal(t, tt.message, body.Message)
			assert.Equal(t, tt.status, StatusCode(tt.err), "StatusCode must agree with WrapError")
		})
	}
}

// A 5xx carries only a message the server's own code wrote; one from a driver or
// a library stays in Origin, for the log.
func TestWrapErrorHidesForeignMessagesOn5xx(t *testing.T) {
	driver := fmt.Errorf(`pq: password authentication failed for user "app" (host=db.internal:5432)`)
	// what a framingo client returns for another service's 404, its category recovered from Kind
	downstream := errors.NotFound.Wrap(&ErrorBody{Source: "users", Status: http.StatusNotFound, Kind: "NotFound", Message: "user u1 not found"})
	tests := []struct {
		name    string
		err     error
		message string
	}{
		{"from another package", driver, "Internal Server Error"},
		{"bare Wrap", errors.Wrap(driver), "Internal Server Error"},
		{"categorized Wrap", errors.DBFailed.Wrap(driver), "Internal Server Error"},
		{"status text of the category", errors.Unavailable.Wrap(driver), "Service Unavailable"},
		{"message-less layers all the way", errors.Internal.Wrap(errors.DBFailed.Wrap(driver)), "Internal Server Error"},
		{"ours, found through another package's wrapper", errors.Wrap(fmt.Errorf("pool: %w", errors.Newf("exhausted"))), "exhausted"},
		{"written", errors.Wrapf(driver, "failed to load user %s", "u1"), "failed to load user u1"},
		{"written under a message-less layer", errors.DBFailed.Wrap(errors.Wrapf(driver, "failed to load user")), "failed to load user"},
		{"written without a cause", errors.Unavailable.Newf("payments down"), "payments down"},
		{"bare category", errors.Unavailable, ""},
		{"no message anywhere", errors.Unavailable.New(errors.WithCode("DOWN", nil)), "Service Unavailable"},
		{"a downstream body absorbed, with a message", errors.Unavailable.Wrapf(errors.Handled(downstream), "call users service"), "call users service"},
		{"a downstream body absorbed, without one", errors.Unavailable.Wrap(errors.Handled(downstream)), "Service Unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := WrapError(tt.err)
			assert.Equal(t, tt.message, body.Message)
			assert.NotEqual(t, http.StatusNotFound, body.Status, "a downstream's category is not the server's")
			assert.Same(t, tt.err, body.Origin, "the log still gets the whole chain")
		})
	}
	assert.Empty(t, PublicMessage(nil, http.StatusInternalServerError))
	// a 4xx keeps its cause's message, which is for the caller to fix the request
	assert.Equal(t, "invalid character 'x'", WrapError(errors.BadRequest.Wrap(fmt.Errorf("invalid character 'x'"))).Message)
}

// A body prints like an xhanio/errors error: the message alone, unless %+v asks
// for the rest.
func TestErrorBodyFormat(t *testing.T) {
	body := &ErrorBody{
		Status: http.StatusTooManyRequests, Kind: "TooManyRequests", Code: "RATE_LIMIT",
		Message: "slow down", Details: labels.Set{"ip": "10.0.0.1"},
	}
	assert.Equal(t, "slow down", fmt.Sprintf("%s", body))
	assert.Equal(t, "slow down", fmt.Sprintf("%v", body))
	assert.Equal(t, "slow down", fmt.Sprintf("%m", body))
	assert.Equal(t, `"slow down"`, fmt.Sprintf("%q", body))
	assert.Equal(t, "call: slow down", fmt.Errorf("call: %w", body).Error())
	assert.Equal(t, "(429) TooManyRequests: slow down {code: RATE_LIMIT, details: ip=10.0.0.1}", fmt.Sprintf("%+v", body))

	assert.Equal(t, "(401) Unauthorized", fmt.Sprintf("%+v", &ErrorBody{Status: http.StatusUnauthorized, Kind: "Unauthorized"}))
	assert.Equal(t, "(502) bad gateway", fmt.Sprintf("%+v", &ErrorBody{Status: http.StatusBadGateway, Message: "bad gateway"}))
	assert.Equal(t, "(400) BadRequest: bad {details: field=name}",
		fmt.Sprintf("%+v", &ErrorBody{Status: http.StatusBadRequest, Kind: "BadRequest", Message: "bad", Details: labels.Set{"field": "name"}}))
}

// echo's verdict on the request is honoured however the error was wrapped,
// unless a category was declared; a wrapped body may be another service's, and
// is not.
func TestWrapErrorHonoursEchosOwnStatus(t *testing.T) {
	bind := echo.NewHTTPError(http.StatusBadRequest, "Syntax error: offset=1, error=invalid character 'x'")
	tests := []struct {
		name    string
		err     error
		status  int
		kind    string
		message string
	}{
		{"wrapped", errors.Wrap(bind), http.StatusBadRequest, "", "Syntax error: offset=1, error=invalid character 'x'"},
		{"wrapped with a message", errors.Wrapf(bind, "invalid user payload"), http.StatusBadRequest, "", "invalid user payload"},
		{"wrapped by another package", fmt.Errorf("bind: %w", bind), http.StatusBadRequest, "", "Syntax error: offset=1, error=invalid character 'x'"},
		{"a 405 wrapped", errors.Wrap(echo.ErrMethodNotAllowed), http.StatusMethodNotAllowed, "", "Method Not Allowed"},
		{"a declared category", errors.BadRequest.Wrap(bind), http.StatusBadRequest, "BadRequest", "Syntax error: offset=1, error=invalid character 'x'"},
		{"a declared Internal", errors.Internal.Wrap(bind), http.StatusInternalServerError, "Internal", "Internal Server Error"},
		{"a body wrapped", errors.Wrap(&ErrorBody{Status: http.StatusNotFound, Kind: "NotFound", Message: "user u1 not found"}),
			http.StatusInternalServerError, "Internal", "Internal Server Error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := WrapError(tt.err)
			assert.Same(t, tt.err, body.Origin)
			assert.Equal(t, tt.status, body.Status)
			assert.Equal(t, tt.kind, body.Kind)
			assert.Equal(t, tt.message, body.Message)
			assert.Equal(t, tt.status, StatusCode(tt.err))
		})
	}
}

// An *echo.HTTPError answers with its own code, 405 included, and StatusCode
// agrees.
func TestWrapErrorKeepsHTTPLayerErrors(t *testing.T) {
	body := &ErrorBody{Status: http.StatusConflict, Kind: "Conflict"}
	assert.Same(t, body, WrapError(body))
	assert.Equal(t, http.StatusConflict, StatusCode(body))

	for _, he := range []*echo.HTTPError{
		echo.NewHTTPError(http.StatusRequestEntityTooLarge, "too big"),
		echo.ErrMethodNotAllowed,
	} {
		got := WrapError(he)
		assert.Same(t, he, got.Origin)
		assert.Equal(t, he.Code, got.Status)
		assert.Equal(t, fmt.Sprint(he.Message), got.Message)
		assert.Equal(t, he.Code, StatusCode(he))
	}
}
