package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

type key struct{}

// SetContext hands ctx to everything below: the request's context is ctx, the rest of the
// request as it was.
func TestSetContextReplacesTheRequestsContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/things?page=2", nil)
	c := echo.New().NewContext(req, httptest.NewRecorder())
	ctx := context.WithValue(req.Context(), key{}, "set")

	SetContext(c, ctx)
	assert.Equal(t, "set", c.Request().Context().Value(key{}))
	assert.Equal(t, "/things?page=2", c.Request().URL.RequestURI())
}
