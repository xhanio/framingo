package server

import (
	"net/http"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/types/api"
)

func TestValidHTTPMethod(t *testing.T) {
	valid := []string{
		http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete, http.MethodOptions,
		http.MethodTrace, http.MethodConnect, api.MethodAny,
	}
	for _, m := range valid {
		assert.True(t, validHTTPMethod(m), "expected %s to be valid", m)
	}

	invalid := []string{"", "INVALID", "any", "get", "PROPFIND"}
	for _, m := range invalid {
		assert.False(t, validHTTPMethod(m), "expected %s to be invalid", m)
	}
}

// A request that fails its validate tags is the caller's mistake: a 400, not
// the 500 an uncategorized error becomes.
func TestValidate_FailedTagIsABadRequest(t *testing.T) {
	type body struct {
		Name string `json:"name" validate:"required"`
	}
	base, cleanup := startServer(t, &mockRouter{
		name: "test",
		config: []byte(`server: http
prefix: /v
handlers:
  - method: POST
    path: /
    func: Create`),
		handlers: map[string]any{"Create": func(c echo.Context) error {
			var b body
			if err := c.Bind(&b); err != nil {
				return err
			}
			if err := c.Validate(&b); err != nil {
				return err
			}
			return c.NoContent(http.StatusCreated)
		}},
	})
	defer cleanup()

	post := func(payload string) int {
		resp, err := http.Post(base+"/v", "application/json", strings.NewReader(payload))
		require.NoError(t, err)
		resp.Body.Close()
		return resp.StatusCode
	}
	assert.Equal(t, http.StatusBadRequest, post(`{}`), "a missing required field")
	assert.Equal(t, http.StatusCreated, post(`{"name":"x"}`))

	err := testManager().newEcho().Validator.Validate("not a struct")
	require.Error(t, err)
	assert.False(t, errors.Is(err, errors.InvalidArgument), "validating a non-struct is the handler's bug, not bad input")
}
