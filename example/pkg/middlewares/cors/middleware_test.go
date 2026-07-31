package cors

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func serve(t *testing.T, method string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	fn, err := New().Func(nil)
	require.NoError(t, err)
	e := echo.New()
	req := httptest.NewRequest(method, "/", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	require.NoError(t, fn(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})(c))
	return rec
}

func TestAllowsAnyOrigin(t *testing.T) {
	rec := serve(t, http.MethodGet, map[string]string{"Origin": "http://localhost:3000"})
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "*", rec.Header().Get(echo.HeaderAccessControlAllowOrigin))
}

func TestAnswersPreflightItself(t *testing.T) {
	rec := serve(t, http.MethodOptions, map[string]string{
		"Origin":                              "http://localhost:3000",
		echo.HeaderAccessControlRequestMethod: http.MethodPost,
	})
	// Preflight is answered by the middleware, not the handler.
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "*", rec.Header().Get(echo.HeaderAccessControlAllowOrigin))
	assert.NotEmpty(t, rec.Header().Get(echo.HeaderAccessControlAllowMethods))
	assert.NotEqual(t, "ok", rec.Body.String())
}

func TestUntouchedWithoutOrigin(t *testing.T) {
	rec := serve(t, http.MethodGet, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get(echo.HeaderAccessControlAllowOrigin))
}

func TestRefusesRouterConfig(t *testing.T) {
	// CORS attaches at server level, before routing; a router.yaml block under
	// its name is a wiring mistake, not something to ignore.
	_, err := New().Func([]byte("allow_origins: ['*']"))
	assert.Error(t, err)
}
