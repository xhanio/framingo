// Package apitest holds test helpers for the example's routers.
package apitest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xhanio/framingo/pkg/services/api/server"
	fapi "github.com/xhanio/framingo/pkg/types/api"

	"github.com/xhanio/framingo/example/pkg/types/rbac"
)

// CheckRouter fails t unless r would register at startup - every func its
// router.yaml names is a handler discovery found, with a signature the method
// fits - and every permission it declares is one rbac defines: a misspelled
// one would lock out every role but admin. It returns the routes declared.
func CheckRouter(t *testing.T, r fapi.Router) []server.Route {
	t.Helper()
	routes, err := server.CheckRouter(r)
	require.NoError(t, err)
	require.NotEmpty(t, routes)
	for _, rt := range routes {
		if rt.Permission != "" {
			assert.True(t, rbac.IsPermissionValid(rt.Permission),
				"%s %s declares permission %q, which rbac does not define", rt.Method, rt.Path, rt.Permission)
		}
	}
	return routes
}
