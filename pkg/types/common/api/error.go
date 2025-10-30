package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/xhanio/errors"
)

const ErrorSourceUnknown = "Unknown"

type ErrorBody struct {
	Origin  error      `json:"-"`                // keep the original error to trace the stack
	Source  string     `json:"source,omitempty"` // source
	Status  int        `json:"status,omitempty"` // http status code
	Code    string     `json:"code,omitempty"`   // customized error code
	Kind    string     `json:"kind,omitempty"`   // error category
	Message string     `json:"message,omitempty"`
	Details labels.Set `json:"details,omitempty"`
}

func (e *ErrorBody) Error() string {
	return e.Message
}

func (e *ErrorBody) Format(f fmt.State, c rune) {
	msg := e.Error()
	switch c {
	case 's':
		fmt.Fprint(f, msg)
	case 'v':
		meta := "{"
		if e.Code != "" {
			meta += fmt.Sprintf("code: %s", e.Code)
		}
		if len(e.Details) > 0 {
			meta += fmt.Sprintf(", details: %s", e.Details)
		}
		meta += "}"
		if len(meta) > 2 {
			msg = fmt.Sprintf("(%d) %s: %s %s", e.Status, e.Kind, msg, meta)
		} else {
			msg = fmt.Sprintf("(%d) %s: %s", e.Status, e.Kind, msg)
		}
		fmt.Fprint(f, msg)
	default:
		fmt.Fprintf(f, "!%%%c(%s)", c, msg)
	}
}

func WrapError(err error, c echo.Context) *ErrorBody {
	switch err {
	case context.Canceled:
		err = errors.Cancaled.Wrap(err)
	case context.DeadlineExceeded:
		err = errors.DeadlineExceeded.Wrap(err)
	}
	switch e := err.(type) {
	case *ErrorBody:
		return e
	case *echo.HTTPError:
		switch e.Code {
		case http.StatusMethodNotAllowed:
			return &ErrorBody{
				Origin:  e,
				Status:  http.StatusNotFound,
				Message: fmt.Sprintf("requested api %s %s not found", c.Request().Method, c.Request().URL.EscapedPath()),
			}
		default:
			return &ErrorBody{
				Origin:  e,
				Status:  e.Code,
				Message: fmt.Sprintf("%v", e.Message),
			}
		}
	case errors.Error:
		status := e.Category().StatusCode()
		code, details := e.Code()
		return &ErrorBody{
			Origin:  e,
			Status:  status,
			Code:    code,
			Kind:    e.Category().Error(),
			Message: e.Message(), // only expose the latest level error message
			Details: details,
		}
	case errors.Category:
		status := e.StatusCode()
		return &ErrorBody{
			Origin: e,
			Status: status,
			Kind:   e.Error(),
		}
	default:
		return &ErrorBody{
			Origin:  e,
			Status:  http.StatusInternalServerError,
			Message: err.Error(),
		}
	}
}

func StatusCode(err error) int {
	if err == nil {
		return http.StatusOK
	}
	switch e := err.(type) {
	case *ErrorBody:
		return e.Status
	case *echo.HTTPError:
		return e.Code
	case errors.Error:
		return e.Category().StatusCode()
	case errors.Category:
		return e.StatusCode()
	default:
		return http.StatusInternalServerError
	}
}
