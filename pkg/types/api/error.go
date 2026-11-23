package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/xhanio/labels"

	"github.com/xhanio/errors"
)

const ErrorSourceUnknown = "Unknown"

type ErrorBody struct {
	Origin  error      `json:"-"` // keep the original error to trace the stack
	Source  string     `json:"source,omitempty"`
	Status  int        `json:"status,omitempty"`
	Code    string     `json:"code,omitempty"` // customized error code
	Kind    string     `json:"kind,omitempty"` // error category
	Message string     `json:"message,omitempty"`
	Details labels.Set `json:"details,omitempty"`
}

func (e *ErrorBody) Error() string {
	return e.Message
}

// Format follows xhanio/errors. %s, %v and %m print Error() and nothing else, so
// a body reads like any other error when printed or wrapped with
// fmt.Errorf("%w"), which renders its operand with %v. %+v adds the status, kind,
// code and details: "(404) NotFound: user u1 not found {code: X, details: k=v}".
// %q prints Error() quoted.
func (e *ErrorBody) Format(f fmt.State, c rune) {
	msg := e.Error()
	switch c {
	case 's', 'm':
		fmt.Fprint(f, msg)
	case 'q':
		fmt.Fprintf(f, "%q", msg)
	case 'v':
		if f.Flag('+') {
			msg = e.verbose()
		}
		fmt.Fprint(f, msg)
	default:
		fmt.Fprintf(f, "!%%%c(%s)", c, msg)
	}
}

func (e *ErrorBody) verbose() string {
	var b strings.Builder
	fmt.Fprintf(&b, "(%d)", e.Status)
	if e.Kind != "" {
		b.WriteString(" " + e.Kind)
	}
	if e.Message != "" {
		if e.Kind != "" {
			b.WriteString(":")
		}
		b.WriteString(" " + e.Message)
	}
	var meta []string
	if e.Code != "" {
		meta = append(meta, "code: "+e.Code)
	}
	if len(e.Details) > 0 {
		meta = append(meta, "details: "+e.Details.String())
	}
	if len(meta) > 0 {
		b.WriteString(" {" + strings.Join(meta, ", ") + "}")
	}
	return b.String()
}

// WrapError builds the response body for a handler's error. Status and Kind come
// from the category the error resolves to, through wrappers from any package: a
// context.Canceled or context.DeadlineExceeded is Canceled or DeadlineExceeded,
// a Combine of several reasons no wrapper declared is Combined, and an error
// without a category is Internal. Where no category was declared, an
// *echo.HTTPError in the chain - echo's own verdict on this request, a failed
// c.Bind's 400 among them - answers with its code, and no kind. An *ErrorBody
// returned as is is the handler's own answer and goes out unchanged; wrapped, it
// may be another service's, which is not honoured. Message is PublicMessage's;
// Origin keeps the whole chain for the log.
func WrapError(err error) *ErrorBody {
	if body, ok := err.(*ErrorBody); ok { //nolint:errorlint // returned as is alone: see the doc
		return body
	}
	code, details := errors.CodeOf(err)
	body := &ErrorBody{Origin: err, Code: code, Details: details}
	var he *echo.HTTPError
	if !errors.Categorized(err) && errors.As(err, &he) {
		body.Status = he.Code
	} else {
		category := errors.CategoryOf(err)
		body.Status, body.Kind = category.StatusCode(), category.Error()
	}
	body.Message = PublicMessage(err, body.Status)
	return body
}

// StatusCode is the status WrapError answers err with, for metrics and spans to
// agree with the committed response.
func StatusCode(err error) int {
	if err == nil {
		return http.StatusOK
	}
	return WrapError(err).Status
}

// PublicMessage is what a response at status tells a client of err: the
// outermost message the server's own code wrote (errors.MessageOf), as
// errors.Wrapf(err, "failed to load user") does, and nothing for a bare
// Category, which the response's kind names. Where the server's code wrote
// none, a 5xx says its status text: the text of the error it received, which
// errors.Wrap(err) would surface, can carry hosts, users or queries. A 4xx is
// the caller's to fix and says that text, which tells them how: an echo error's
// message, a failed validation's fields.
func PublicMessage(err error, status int) string {
	if err == nil {
		return ""
	}
	if _, bare := err.(errors.Category); bare { //nolint:errorlint // a bare category returned as is says nothing past its name
		return ""
	}
	if msg := errors.MessageOf(err); msg != "" {
		return msg
	}
	if status >= http.StatusInternalServerError {
		return http.StatusText(status)
	}
	var he *echo.HTTPError
	if errors.As(err, &he) {
		return fmt.Sprintf("%v", he.Message)
	}
	return err.Error()
}
