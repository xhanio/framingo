package authz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/xhanio/errors"

	fapi "github.com/xhanio/framingo/pkg/types/api"

	"github.com/xhanio/framingo/example/pkg/services/system/role"
	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/preset"
)

// fakeRoles implements only HasPermission; any other role.Manager call panics.
type fakeRoles struct {
	role.Manager
	grants map[string][]string
}

func (f fakeRoles) HasPermission(_ context.Context, roleName, permission string) (bool, error) {
	for _, p := range f.grants[roleName] {
		if p == permission {
			return true, nil
		}
	}
	return false, nil
}

func serve(t *testing.T, cred *entity.Credential, permission string) (bool, error) {
	t.Helper()
	mw, err := New(fakeRoles{grants: map[string][]string{"reader": {"thing_read"}}}).Func(true, nil)
	if err != nil {
		t.Fatal(err)
	}
	c := echo.New().NewContext(httptest.NewRequest(http.MethodGet, "/things", nil), httptest.NewRecorder())
	if cred != nil {
		fapi.SetContext(c, entity.WithCredential(c.Request().Context(), cred))
	}
	c.Set(fapi.ContextKeyRequestInfo, &fapi.RequestInfo{Permission: permission})
	reached := false
	err = mw(func(echo.Context) error { reached = true; return nil })(c)
	return reached, err
}

func TestAuthz(t *testing.T) {
	user := &entity.Credential{Source: preset.AuthSourceLocalUser, Role: "reader"}
	isA := func(category errors.Category, err error) bool { return err != nil && errors.Is(err, category) }

	reached, err := serve(t, user, "thing_read")
	assert.NoError(t, err)
	assert.True(t, reached, "granted permission")

	reached, err = serve(t, user, "thing_write")
	assert.True(t, isA(errors.Forbidden, err), "missing permission: %v", err)
	assert.False(t, reached)

	reached, err = serve(t, user, "")
	assert.True(t, isA(errors.Forbidden, err), "no permission declared: %v", err)
	assert.False(t, reached)

	reached, err = serve(t, nil, "thing_read")
	assert.True(t, isA(errors.Unauthorized, err), "no credential: %v", err)
	assert.False(t, reached)

	reset := &entity.Credential{Source: preset.AuthSourceLocalUser, Role: "reader", RequirePasswordReset: true}
	reached, err = serve(t, reset, "thing_read")
	assert.True(t, isA(errors.Forbidden, err), "password reset pending: %v", err)
	assert.False(t, reached)
}

func TestAuthzSwitchedOff(t *testing.T) {
	mw, err := New(fakeRoles{}).Func(false, nil)
	assert.NoError(t, err)
	assert.Nil(t, mw)
}

func TestNameDefaultsToPackage(t *testing.T) {
	assert.Equal(t, "authz", New(nil).Name())
}

func TestWithNameOverrides(t *testing.T) {
	assert.Equal(t, "authz2", New(nil, WithName("authz2")).Name())
	assert.Equal(t, "authz", New(nil, WithName("")).Name())
}
