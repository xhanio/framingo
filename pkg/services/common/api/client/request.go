package client

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/types/common"
)

type Request struct {
	Method      string
	Path        string
	Headers     common.Pairs
	Cookies     []*http.Cookie
	ContentType string
	Body        any
}

func (r *Request) ParseBody() (io.Reader, error) {
	if body, ok := r.Body.(io.Reader); ok {
		return body, nil
	}
	if b, ok := r.Body.([]byte); ok {
		return bytes.NewReader(b), nil
	}
	if s, ok := r.Body.(string); ok {
		return strings.NewReader(s), nil
	}
	switch r.ContentType {
	case echo.MIMEApplicationJSON:
		b, err := json.Marshal(r.Body)
		if err != nil {
			return nil, errors.Wrap(err)
		}
		return bytes.NewReader(b), nil
	default:
		return nil, nil
	}
}
