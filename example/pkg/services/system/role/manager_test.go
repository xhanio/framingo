package role

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/utils/testutil"

	"github.com/xhanio/framingo/example/pkg/services/repository"
	"github.com/xhanio/framingo/example/pkg/services/system/platform"
	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/orm"
	"github.com/xhanio/framingo/example/pkg/types/preset"
	"github.com/xhanio/framingo/example/pkg/types/rbac"
	"github.com/xhanio/framingo/example/pkg/types/repo"
)

var createOpts = entity.RoleCreateOptions{Name: "test", Description: "Test Role"}

// the three seeded roles take IDs 1-3
const testRoleID = int32(4)

// setup returns an initialized service and the same repository as repo.User,
// for writes the role service cannot make - a user holding a role.
func setup(t *testing.T) (*manager, repo.User) {
	t.Helper()
	db, err := testutil.SetupDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Cleanup(true)) })
	require.NoError(t, db.ORM().AutoMigrate(
		&orm.Organization{},
		&orm.Role{},
		&orm.RolePermission{},
		&orm.Contact{},
		&orm.User{},
		&orm.Certificate{},
	))
	p := platform.New(db, nil, nil, nil)
	r := repository.New(p)
	m := newManager(p, r)
	require.NoError(t, m.Init(context.Background()))
	return m, r
}

func isA(category errors.Category) func(error) bool {
	return func(err error) bool { return err != nil && errors.Is(err, category) }
}

func TestCreate(t *testing.T) {
	m, _ := setup(t)
	ctx := context.Background()

	roles, err := m.List(ctx)
	require.NoError(t, err)
	assert.Len(t, roles, 3, "the seeded roles")

	require.NoError(t, m.Create(ctx, createOpts))
	roles, err = m.List(ctx)
	require.NoError(t, err)
	assert.Len(t, roles, 4)
	role, err := m.GetByName(ctx, createOpts.Name)
	require.NoError(t, err)
	assert.Equal(t, testRoleID, role.ID)
	assert.Equal(t, createOpts.Description, role.Description)
}

// A role users still hold is not deleted - nor are its permissions - and one
// nobody holds is.
func TestDelete(t *testing.T) {
	m, users := setup(t)
	ctx := context.Background()

	assert.True(t, isA(errors.NotFound)(m.Delete(ctx, 10)), "an unknown role")

	require.NoError(t, m.Create(ctx, createOpts))
	require.NoError(t, m.SetPermissions(ctx, testRoleID, entity.PermissionSetOptions{Permissions: []string{rbac.PermissionUserRead}}))
	holder, err := users.CreateUser(ctx, repo.UserCreateOptions{
		OrganizationID: preset.DefaultOrganizationID,
		Username:       "holder",
		PasswordHash:   "hash",
		Role:           createOpts.Name,
	})
	require.NoError(t, err)

	err = m.Delete(ctx, testRoleID)
	assert.True(t, isA(errors.BadRequest)(err), "a role in use: got %v", err)
	perms, err := m.GetPermissions(ctx, testRoleID)
	require.NoError(t, err)
	assert.Len(t, perms, 1, "a refused delete leaves the role's permissions")

	require.NoError(t, users.DeleteUsers(ctx, []int32{holder.ID}))
	require.NoError(t, m.Delete(ctx, testRoleID))
	_, err = m.GetByName(ctx, createOpts.Name)
	assert.True(t, isA(errors.NotFound)(err), "the deleted role: got %v", err)
}

func TestPermissions(t *testing.T) {
	m, _ := setup(t)
	ctx := context.Background()
	require.NoError(t, m.Create(ctx, createOpts))

	require.NoError(t, m.repository.AddRolePermission(ctx, testRoleID, rbac.PermissionUserRead))
	perms, err := m.GetPermissions(ctx, testRoleID)
	require.NoError(t, err)
	assert.Equal(t, []string{rbac.PermissionUserRead}, perms)
	perms, err = m.GetPermissionsByName(ctx, createOpts.Name)
	require.NoError(t, err)
	assert.Equal(t, []string{rbac.PermissionUserRead}, perms)

	require.NoError(t, m.SetPermissions(ctx, testRoleID, entity.PermissionSetOptions{Permissions: rbac.PermissionsAll}))
	perms, err = m.GetPermissions(ctx, testRoleID)
	require.NoError(t, err)
	assert.ElementsMatch(t, rbac.PermissionsAll, perms, "SetPermissions replaces the list")

	perms, err = m.GetPermissions(ctx, rbac.RoleAdminID)
	require.NoError(t, err)
	assert.Equal(t, rbac.PermissionsAll, perms, "the admin role holds every permission implicitly")
}

// The admin role is fixed whoever calls, and a permission name must be one
// rbac defines.
func TestAdminRoleAndPermissionNamesAreGuarded(t *testing.T) {
	m, _ := setup(t)
	ctx := context.Background()
	badRequest := isA(errors.BadRequest)

	name := "renamed"
	err := m.Update(ctx, rbac.RoleAdminID, entity.RoleUpdateOptions{Name: &name})
	assert.True(t, badRequest(err), "updating the admin role: got %v", err)
	err = m.Delete(ctx, rbac.RoleAdminID)
	assert.True(t, badRequest(err), "deleting the admin role: got %v", err)
	err = m.SetPermissions(ctx, rbac.RoleAdminID, entity.PermissionSetOptions{})
	assert.True(t, badRequest(err), "setting the admin role's permissions: got %v", err)
	err = m.SetPermissionsForRoleName(ctx, rbac.RoleAdmin, entity.PermissionSetOptions{})
	assert.True(t, badRequest(err), "setting the admin role's permissions by name: got %v", err)
	admin, err := m.GetByName(ctx, rbac.RoleAdmin)
	require.NoError(t, err)
	assert.Equal(t, rbac.RoleAdmin, admin.Name, "a refused update leaves the admin role as it was")

	require.NoError(t, m.Create(ctx, createOpts))
	bogus := entity.PermissionSetOptions{Permissions: []string{rbac.PermissionUserRead, "no_such_permission"}}
	err = m.SetPermissions(ctx, testRoleID, bogus)
	assert.True(t, badRequest(err), "setting an undefined permission: got %v", err)
	err = m.SetPermissionsForRoleName(ctx, createOpts.Name, bogus)
	assert.True(t, badRequest(err), "setting an undefined permission by name: got %v", err)
	perms, err := m.GetPermissions(ctx, testRoleID)
	require.NoError(t, err)
	assert.Empty(t, perms, "a refused permission list is not partly stored")
}
