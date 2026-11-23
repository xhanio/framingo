package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/utils/log"
)

// A handler that panics with an error answers with that error's category, even
// when another package wrapped it before the panic.
func TestRecoverKeepsAPanickedErrorsCategory(t *testing.T) {
	tests := []struct {
		name     string
		panicked any
		category errors.Category
		message  string
	}{
		{"xhanio/errors error", errors.Conflict.Newf("taken"), errors.Conflict, "!! recover from panic: taken"},
		{"wrapped by another package", fmt.Errorf("save: %w", errors.Conflict.Newf("taken")), errors.Conflict, "!! recover from panic: save: taken"},
		{"bare category", errors.Forbidden, errors.Forbidden, "!! recover from panic: Forbidden"},
		{"not an error", "boom", errors.Internal, "!! recover from panic: boom"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			var got error
			e.HTTPErrorHandler = func(err error, _ echo.Context) { got = err }
			mw := newMiddleware(&server{log: log.New(log.WithLevel(-1))})
			c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())

			err := mw.Recover(func(echo.Context) error { panic(tt.panicked) })(c)
			require.NoError(t, err)
			require.Error(t, got)
			assert.True(t, errors.Is(got, tt.category), "category of %v", got)
			assert.Equal(t, tt.message, got.Error())
		})
	}
}
