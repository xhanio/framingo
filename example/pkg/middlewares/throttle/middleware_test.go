package throttle

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhanio/errors"
	fapi "github.com/xhanio/framingo/pkg/types/api"
)

// funcOf builds one attachment the way the server does at registration.
func funcOf(t *testing.T, m fapi.Middleware, config []byte) echo.MiddlewareFunc {
	t.Helper()
	fn, err := m.Func(config)
	require.NoError(t, err)
	return fn
}

// serve runs one request through mw with the given RequestInfo already on the
// context, the way the built-in Info middleware leaves it for route
// middlewares.
func serve(t *testing.T, mw echo.MiddlewareFunc, req *fapi.RequestInfo) error {
	t.Helper()
	e := echo.New()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())
	c.Set(fapi.ContextKeyRequestInfo, req)
	return mw(func(c echo.Context) error { return nil })(c)
}

func info(ip string) *fapi.RequestInfo {
	return &fapi.RequestInfo{IP: ip, Path: "/a"}
}

func TestLimit(t *testing.T) {
	mw := funcOf(t, New(), []byte("rps: 1\nburst_size: 2\n"))

	// The burst passes, the request after it is limited.
	require.NoError(t, serve(t, mw, info("1.2.3.4")))
	require.NoError(t, serve(t, mw, info("1.2.3.4")))
	err := serve(t, mw, info("1.2.3.4"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.TooManyRequests), "expected TooManyRequests, got %v", err)
}

func TestLimitIsPerIP(t *testing.T) {
	mw := funcOf(t, New(), []byte("rps: 1\nburst_size: 1\n"))

	require.NoError(t, serve(t, mw, info("1.2.3.4")))
	require.Error(t, serve(t, mw, info("1.2.3.4")))
	// A different IP has its own limiter.
	assert.NoError(t, serve(t, mw, info("5.6.7.8")))
}

func TestAttachmentsAreIndependent(t *testing.T) {
	m := New()
	limit := []byte("rps: 1\nburst_size: 1\n")
	a := funcOf(t, m, limit)
	b := funcOf(t, m, limit)

	// Each attachment - each route - keeps its own limiter table, so
	// exhausting one route does not starve another.
	require.NoError(t, serve(t, a, info("1.2.3.4")))
	require.Error(t, serve(t, a, info("1.2.3.4")))
	assert.NoError(t, serve(t, b, info("1.2.3.4")))
}

func TestNoConfigMeansNoThrottle(t *testing.T) {
	// A bare attachment on a server with no default for this middleware.
	mw := funcOf(t, New(), nil)
	for range 10 {
		assert.NoError(t, serve(t, mw, info("1.2.3.4")))
	}
}

func TestZeroConfigMeansNoThrottle(t *testing.T) {
	// The old server.WithThrottle treated a zero rps or burst as "no
	// throttle"; the config keeps that contract.
	mw := funcOf(t, New(), []byte("rps: 0\nburst_size: 0\n"))
	for range 5 {
		assert.NoError(t, serve(t, mw, info("1.2.3.4")))
	}
}

func TestBadConfigFailsToBuild(t *testing.T) {
	_, err := New().Func([]byte("rps: [not, a, number]"))
	assert.Error(t, err)
}

func TestMissingRequestInfoIsAnError(t *testing.T) {
	mw := funcOf(t, New(), []byte("rps: 1\nburst_size: 1\n"))
	e := echo.New()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())
	err := mw(func(c echo.Context) error { return nil })(c)
	assert.Error(t, err)
}

func TestPartialConfigMeansNoThrottle(t *testing.T) {
	// A block with only rps and no burst_size leaves burst at zero, which the
	// zeros contract reads as unthrottled - surprising enough to pin.
	mw := funcOf(t, New(), []byte("rps: 5\n"))
	for range 100 {
		assert.NoError(t, serve(t, mw, info("1.2.3.4")))
	}
}
