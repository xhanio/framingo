package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/common/api"
	"github.com/xhanio/framingo/pkg/utils/log"
)

type client struct {
	log log.Logger

	endpoint  *api.Endpoint
	tlsConfig *api.ClientTLS

	debug   bool
	timeout time.Duration

	headers map[string]string
	cookies map[string]*http.Cookie
	cli     *http.Client
}

func New(endpoint string, opts ...Option) Client {
	return newClient(endpoint, opts...)
}

func newClient(endpoint string, opts ...Option) *client {
	c := &client{
		endpoint: &api.Endpoint{
			Host: endpoint,
		},
		headers: make(map[string]string),
		cookies: make(map[string]*http.Cookie),
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.log == nil {
		c.log = log.Default
	}
	return c
}

func (c *client) Init() error {
	if c.endpoint == nil {
		return errors.Newf("failed to init client: no endpoint specified")
	}
	endpoint := c.endpoint.String()
	if strings.HasPrefix(endpoint, "https://") {
		c.cli = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig:     c.tlsConfig.AsConfig(),
				TLSHandshakeTimeout: 2 * time.Second,
			},
			Timeout: c.timeout,
		}
	} else {
		c.cli = &http.Client{
			Timeout: c.timeout,
		}
	}
	return nil
}

func (c *client) SetHeaders(headers ...common.Pair[string, string]) {
	for _, header := range headers {
		if header.GetKey() == "" {
			continue
		}
		if header.GetValue() != "" {
			c.headers[header.GetKey()] = header.GetValue()
		} else {
			delete(c.headers, header.GetKey())
		}
	}
}

func (c *client) SetCookies(cookies ...*http.Cookie) {
	for _, cookie := range cookies {
		if cookie.Name == "" {
			continue
		}
		if cookie.Value != "" {
			c.cookies[cookie.Name] = cookie
		} else {
			delete(c.cookies, cookie.Name)
		}
	}
}

func (c *client) NewRequest(ctx context.Context, request *Request) (*http.Request, error) {
	url := fmt.Sprintf("%s/%s", strings.TrimSuffix(c.endpoint.String(), "/"), strings.TrimPrefix(request.Path, "/"))
	body, err := request.ParseBody()
	if err != nil {
		return nil, errors.Wrap(err)
	}
	r, err := http.NewRequestWithContext(ctx, request.Method, url, body)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	for key, val := range c.headers {
		// apply global headers
		r.Header.Set(key, val)
	}
	for _, header := range request.Headers {
		r.Header.Set(header.GetKey(), header.GetValue())
	}
	for _, cookie := range c.cookies {
		// apply global cookies
		r.AddCookie(cookie)
	}
	for _, cookie := range request.Cookies {
		r.AddCookie(cookie)
	}
	if request.ContentType != "" {
		r.Header.Set("Content-Type", request.ContentType)
	}
	return r, nil
}

func (c *client) Do(req *http.Request) (*http.Response, error) {
	c.log.Debugf("%s %s", req.Method, req.URL)
	if c.debug {
		c.log.Debug("url:", req.URL.String())
		c.log.Debug("headers:")
		for key := range req.Header {
			c.log.Debugf("\t%s: %v\n", key, req.Header[key])
		}
	}
	resp, err := c.cli.Do(req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	if c.debug {
		c.log.Debugf("response code: %d\n", resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
			return nil, errors.Wrapf(err, "failed to read response body")
		}
		resp.Body.Close()
		// !!! overwrite resp body by an empty readcloser
		// so that outside call could do "defer resp.Body.Close()" anyway
		resp.Body = io.NopCloser(bytes.NewReader(nil))
		var body api.ErrorBody
		if err := json.Unmarshal(b, &body); err != nil || body.Source == "" {
			// resp body is not a valid json
			// or the source of the error is unknown
			return resp, &api.ErrorBody{
				Source:  api.ErrorSourceUnknown,
				Status:  resp.StatusCode,
				Message: string(b),
			}
		}
		return resp, &body
	}
	return resp, nil
}

func (c *client) Get(ctx context.Context, path string, opts ...RequestOption) (*http.Response, error) {
	r := &Request{
		Method: http.MethodGet,
		Path:   path,
	}
	for _, opt := range opts {
		opt(r)
	}
	req, err := c.NewRequest(ctx, r)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *client) PostJSON(ctx context.Context, path string, body any, opts ...RequestOption) (*http.Response, error) {
	r := &Request{
		Method:      http.MethodPost,
		Path:        path,
		ContentType: echo.MIMEApplicationJSON,
		Body:        body,
	}
	for _, opt := range opts {
		opt(r)
	}
	req, err := c.NewRequest(ctx, r)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
