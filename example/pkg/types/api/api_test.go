package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fapi "github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/utils/confutil"
)

type key struct{}

// A Context answers Deadline, Done, Err and a typed key's Value from the request it was
// made for, once echo reset its pooled echo.Context for the next request: database/sql
// asks Done and Err from a goroutine of its own when a query's rows close.
func TestAContextOutlivesItsPooledEchoContext(t *testing.T) {
	e := echo.New()
	deadline := time.Now().Add(time.Hour)
	request, cancel := context.WithDeadline(context.WithValue(context.Background(), key{}, "first"), deadline)
	ec := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil).WithContext(request), httptest.NewRecorder())
	var held Context
	require.NoError(t, WrapHandler(func(c Context) error {
		held = c
		return nil
	})(ec))

	ec.Reset(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder()) // as echo does for the next request
	cancel()
	assert.ErrorIs(t, held.Err(), context.Canceled, "the request it was made for, not the next one")
	select {
	case <-held.Done():
	default:
		t.Error("Done is the next request's")
	}
	got, ok := held.Deadline()
	assert.True(t, ok && got.Equal(deadline), "the deadline is the next request's")
	assert.Equal(t, "first", held.Value(key{}))
}

// A string key echo's store does not hold is answered from the request's context, where a
// middleware may have put one with context.WithValue, as confutil.WrapContext and
// db.WrapContext do; one the store holds is the store's.
func TestAContextAnswersAStringKeyTheStoreLacksFromTheRequest(t *testing.T) {
	e := echo.New()
	config := confutil.New(viper.New())
	request := confutil.WrapContext(context.Background(), config)
	ec := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil).WithContext(request), httptest.NewRecorder())
	ec.Set(fapi.ContextKeyTrace, "trace")
	var held Context
	require.NoError(t, WrapHandler(func(c Context) error {
		held = c
		return nil
	})(ec))

	assert.Same(t, config, confutil.FromContext(held), "the request's, under a string key")
	assert.Equal(t, "trace", held.Value(fapi.ContextKeyTrace), "the store's")
}
