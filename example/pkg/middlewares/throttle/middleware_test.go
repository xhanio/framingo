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

func TestInstanceLimit(t *testing.T) {
	mw := funcOf(t, New(WithLimit(1, 2)), nil)

	// The burst passes, the request after it is limited.
	require.NoError(t, serve(t, mw, info("1.2.3.4")))
	require.NoError(t, serve(t, mw, info("1.2.3.4")))
	err := serve(t, mw, info("1.2.3.4"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.TooManyRequests), "expected TooManyRequests, got %v", err)
}

func TestLimitIsPerIP(t *testing.T) {
	mw := funcOf(t, New(WithLimit(1, 1)), nil)

	require.NoError(t, serve(t, mw, info("1.2.3.4")))
	require.Error(t, serve(t, mw, info("1.2.3.4")))
	// A different IP has its own limiter.
	assert.NoError(t, serve(t, mw, info("5.6.7.8")))
}

func TestConfigOverridesInstanceLimit(t *testing.T) {
	m := New(WithLimit(1, 100))
	// The instance limit alone would allow the burst; the route's config
	// tightens it to a single request.
	mw := funcOf(t, m, []byte("rps: 1\nburst_size: 1\n"))

	require.NoError(t, serve(t, mw, info("1.2.3.4")))
	assert.Error(t, serve(t, mw, info("1.2.3.4")))
}

func TestConfigWithoutInstanceLimit(t *testing.T) {
	mw := funcOf(t, New(), []byte("rps: 1\nburst_size: 1\n"))

	require.NoError(t, serve(t, mw, info("1.2.3.4")))
	assert.Error(t, serve(t, mw, info("1.2.3.4")))
}

func TestAttachmentsAreIndependent(t *testing.T) {
	m := New(WithLimit(1, 1))
	a := funcOf(t, m, nil)
	b := funcOf(t, m, nil)

	// Each attachment - each route - keeps its own limiter table, so
	// exhausting one route does not starve another.
	require.NoError(t, serve(t, a, info("1.2.3.4")))
	require.Error(t, serve(t, a, info("1.2.3.4")))
	assert.NoError(t, serve(t, b, info("1.2.3.4")))
}

func TestNoLimitMeansNoThrottle(t *testing.T) {
	mw := funcOf(t, New(), nil)
	for range 10 {
		assert.NoError(t, serve(t, mw, info("1.2.3.4")))
	}
}

func TestZeroLimitMeansNoThrottle(t *testing.T) {
	// The old server.WithThrottle treated a zero rps or burst as "no
	// throttle"; WithLimit and the config keep that contract.
	mw := funcOf(t, New(WithLimit(0, 100)), nil)
	assert.NoError(t, serve(t, mw, info("1.2.3.4")))

	mw = funcOf(t, New(WithLimit(1, 1)), []byte("rps: 0\nburst_size: 0\n"))
	for range 5 {
		assert.NoError(t, serve(t, mw, info("1.2.3.4")))
	}
}

func TestBadConfigFailsToBuild(t *testing.T) {
	_, err := New().Func([]byte("rps: [not, a, number]"))
	assert.Error(t, err)
}

func TestMissingRequestInfoIsAnError(t *testing.T) {
	mw := funcOf(t, New(WithLimit(1, 1)), nil)
	e := echo.New()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())
	err := mw(func(c echo.Context) error { return nil })(c)
	assert.Error(t, err)
}
