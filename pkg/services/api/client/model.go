package client

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"
)

type Request struct {
	Method      string
	Path        string
	Headers     common.Pairs
	Cookies     []*http.Cookie
	ContentType string
	Body        any
	Encoding api.Encoding
}

func (r *Request) ParseBody() (io.Reader, error) {
	reader, err := r.parseRawBody()
	if err != nil {
		return nil, errors.Wrap(err)
	}
	if r.Encoding == "" || reader == nil {
		return reader, nil
	}
	switch r.Encoding {
	case api.EncodingDeflate:
		var buf bytes.Buffer
		zw := zlib.NewWriter(&buf)
		if _, err := io.Copy(zw, reader); err != nil {
			return nil, errors.Wrap(err)
		}
		if err := zw.Close(); err != nil {
			return nil, errors.Wrap(err)
		}
		return &buf, nil
	default:
		return nil, errors.BadRequest.Newf("unsupported encoding: %s", r.Encoding)
	}
}

func (r *Request) parseRawBody() (io.Reader, error) {
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

type Client interface {
	common.Initializable
	SetHeaders(headers ...common.Pair[string, string])
	SetCookies(cookies ...*http.Cookie)
	NewRequest(ctx context.Context, request *Request, opts ...RequestOption) (*http.Request, error)
	Do(req *http.Request) (*http.Response, error)
	Send(ctx context.Context, request *Request, opts ...RequestOption) (*http.Response, error)
}
