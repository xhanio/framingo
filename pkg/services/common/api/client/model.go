package client

import (
	"context"
	"net/http"

	"github.com/xhanio/framingo/pkg/types/common"
)

type Client interface {
	common.Initializable
	SetHeaders(headers ...common.Pair[string, string])
	SetCookies(cookies ...*http.Cookie)
	NewRequest(ctx context.Context, request *Request) (*http.Request, error)
	Do(req *http.Request) (*http.Response, error)
	Get(ctx context.Context, url string, opts ...RequestOption) (*http.Response, error)
	PostJSON(ctx context.Context, url string, body any, opts ...RequestOption) (*http.Response, error)
}
