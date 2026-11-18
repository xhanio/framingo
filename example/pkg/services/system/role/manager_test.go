package role

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/types/model"
	"github.com/xhanio/framingo/pkg/utils/testutil"

	"github.com/xhanio/framingo/example/pkg/services/repository"
	"github.com/xhanio/framingo/example/pkg/services/system/platform"
	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/orm"
	"github.com/xhanio/framingo/example/pkg/types/preset"
	"github.com/xhanio/framingo/example/pkg/types/rbac"
	"github.com/xhanio/framingo/example/pkg/types/repo"
)

var (
	createOpts = entity.RoleCreateOptions{
		Name:        "test",
		Description: "Test Role",
	}

	testRoleID     = int32(4)
	notExistRoleID = int32(10)
)

func setup() (*manager, model.DatabaseDebugger, error) {
	m, db, _, err := setupWithUsers()
	return m, db, err
}

func setupWithUsers() (*manager, model.DatabaseDebugger, repo.User, error) {
	db, err := testutil.SetupDB()
	if err != nil {
		return nil, nil, nil, errors.Wrap(err)
	}
	if err := db.ORM().AutoMigrate(
		&orm.Organization{},
		&orm.Role{},
		&orm.RolePermission{},
		&orm.Contact{},
		&orm.User{},
		&orm.Certificate{},
	); err != nil {
		return nil, nil, nil, errors.Wrap(err)
	}
	p := platform.New(db, nil, nil, nil)
	r := repository.New(p)
	m := newManager(p, r)
	if err := m.Init(context.Background()); err != nil {
		return nil, nil, nil, errors.Wrap(err)
	}
	return m, db, r, nil
}

func TestCreateList(t *testing.T) {
	m, db, err := setup()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Cleanup(true); err != nil {
			t.Fatalf("Error cleaning up dbManager: %v", err)
		}
	}()

	if roles, err := m.List(context.Background()); err != nil {
		t.Fatalf("Error from List Role: %s", err.Error())
	} else if len(roles) != 3 {
		t.Errorf("Expected number of roles: got = %v; expected %v", len(roles), 3)
	}

	if err := m.Create(context.Background(), createOpts); err != nil {
		t.Fatalf("Error from Create role: %v", err.Error())
	}
	if roles, err := m.List(context.Background()); err != nil {
		t.Fatalf("Error from List roles: %v", err.Error())
	} else if len(roles) != 4 {
		t.Errorf("Expected number of roles: got = %v; expected %v", len(roles), 4)
	} else {
		createdRole := roles[3]
		if role, err := m.repository.GetRole(context.Background(), createdRole.ID); err != nil {
			t.Fatalf("Failed to get the role: %v", err.Error())
		} else if role.Name != createOpts.Name {
			t.Fatalf("Expected role name: got = %v; expected %v", role.Name, createOpts.Name)
		}
	}
}

func TestCreateGet(t *testing.T) {
	m, db, err := setup()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Cleanup(true); err != nil {
			t.Fatalf("Error cleaning up dbManager: %v", err)
		}
	}()

	if err := m.Create(context.Background(), createOpts); err != nil {
		t.Fatalf("Error from Create role: %v", err.Error())
	}
	if role, err := m.repository.GetRoleByName(context.Background(), createOpts.Name); err != nil {
		t.Fatalf("Error from List roles: %v", err.Error())
	} else if role.Name != createOpts.Name {
		t.Errorf("Expected role name: got = %v; expected %v", role.Name, createOpts.Name)
	}
}

func TestDelete(t *testing.T) {
	m, db, err := setup()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Cleanup(true); err != nil {
			t.Fatalf("Error cleaning up dbManager: %v", err)
		}
	}()

	if err := m.Delete(context.Background(), notExistRoleID); err != nil {
		if be, ok := err.(errors.Error); ok {
			if !errors.Is(be, errors.NotFound) {
				t.Fatalf("Expected error from Delete User: %v, but got: %v", errors.NotFound, err.Error())
			}
		}
	} else {
		t.Fatalf("Expecting error from deleting not existing user")
	}
}

func TestAddPermission(t *testing.T) {
	m, db, err := setup()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Cleanup(true); err != nil {
			t.Fatalf("Error cleaning up dbManager: %v", err)
		}
	}()

	if err := m.Create(context.Background(), createOpts); err != nil {
		t.Fatalf("Error from Create role: %v", err.Error())
	}

	if err := m.repository.AddRolePermission(context.Background(), testRoleID, rbac.PermissionUserRead); err != nil {
		t.Fatalf("Error from Create role: %v", err.Error())
	}

	if perms, err := m.GetPermissions(context.Background(), testRoleID); err != nil {
		t.Fatalf("Error from List roles: %v", err.Error())
	} else if len(perms) != 1 {
		t.Errorf("Expected number of roles: got = %v; expected %v", len(perms), 1)
	} else if perms[0] != rbac.PermissionUserRead {
		t.Errorf("Expected permission: got = %v; expected %v", perms[0], rbac.PermissionUserRead)
	}
}

func TestGetPermissionByName(t *testing.T) {
	m, db, err := setup()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Cleanup(true); err != nil {
			t.Fatalf("Error cleaning up dbManager: %v", err)
		}
	}()

	if err := m.Create(context.Background(), createOpts); err != nil {
		t.Fatalf("Error from Create role: %v", err.Error())
	}

	if err := m.repository.AddRolePermission(context.Background(), testRoleID, rbac.PermissionUserRead); err != nil {
		t.Fatalf("Error from Create role: %v", err.Error())
	}

	if perms, err := m.GetPermissionsByName(context.Background(), createOpts.Name); err != nil {
		t.Fatalf("Error from List roles: %v", err.Error())
	} else if len(perms) != 1 {
		t.Errorf("Expected number of roles: got = %v; expected %v", len(perms), 1)
	} else if perms[0] != rbac.PermissionUserRead {
		t.Errorf("Expected permission: got = %v; expected %v", perms[0], rbac.PermissionUserRead)
	}
}

func TestSetPermission(t *testing.T) {
	m, db, err := setup()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Cleanup(true); err != nil {
			t.Fatalf("Error cleaning up dbManager: %v", err)
		}
	}()

	if err := m.Create(context.Background(), createOpts); err != nil {
		t.Fatalf("Error from Create role: %v", err.Error())
	}

	opts := entity.PermissionSetOptions{
		Permissions: rbac.PermissionsAll,
	}

	if err := m.SetPermissions(context.Background(), int32(testRoleID), opts); err != nil {
		t.Fatalf("Error from setting permissions for role: %v", err.Error())
	}
	if perms, err := m.GetPermissions(context.Background(), testRoleID); err != nil {
		t.Fatalf("Error from List roles: %v", err.Error())
	} else if len(perms) != len(rbac.PermissionsAll) {
		t.Errorf("Expected number of roles: got = %v; expected %v", len(perms), 1)
	}

}

func TestDeleteRefusesARoleInUse(t *testing.T) {
	m, db, users, err := setupWithUsers()
	require.NoError(t, err)
	defer func() { require.NoError(t, db.Cleanup(true)) }()
	ctx := context.Background()

	require.NoError(t, m.Create(ctx, createOpts))
	role, err := m.GetByName(ctx, createOpts.Name)
	require.NoError(t, err)
	holder, err := users.CreateUser(ctx, repo.UserCreateOptions{
		OrganizationID: preset.DefaultOrganizationID,
		Username:       "holder",
		PasswordHash:   "hash",
		Role:           createOpts.Name,
	})
	require.NoError(t, err)

	err = m.Delete(ctx, role.ID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.BadRequest), "got %v", err)
	_, err = m.GetByName(ctx, createOpts.Name)
	assert.NoError(t, err, "a refused delete leaves the role")

	require.NoError(t, users.DeleteUsers(ctx, []int32{holder.ID}))
	require.NoError(t, m.Delete(ctx, role.ID))
	_, err = m.GetByName(ctx, createOpts.Name)
	assert.True(t, errors.Is(err, errors.NotFound), "got %v", err)
}

func TestAdminRoleAndPermissionNamesAreGuarded(t *testing.T) {
	m, db, err := setup()
	require.NoError(t, err)
	defer func() { require.NoError(t, db.Cleanup(true)) }()
	ctx := context.Background()
	isBadRequest := func(err error) bool { return err != nil && errors.Is(err, errors.BadRequest) }

	name := "renamed"
	err = m.Update(ctx, rbac.RoleAdminID, entity.RoleUpdateOptions{Name: &name})
	assert.True(t, isBadRequest(err), "updating the admin role: got %v", err)
	err = m.Delete(ctx, rbac.RoleAdminID)
	assert.True(t, isBadRequest(err), "deleting the admin role: got %v", err)
	err = m.SetPermissions(ctx, rbac.RoleAdminID, entity.PermissionSetOptions{})
	assert.True(t, isBadRequest(err), "setting the admin role's permissions: got %v", err)
	err = m.SetPermissionsForRoleName(ctx, rbac.RoleAdmin, entity.PermissionSetOptions{})
	assert.True(t, isBadRequest(err), "setting the admin role's permissions by name: got %v", err)
	admin, err := m.GetByName(ctx, rbac.RoleAdmin)
	require.NoError(t, err)
	assert.Equal(t, rbac.RoleAdmin, admin.Name, "a refused update leaves the admin role as it was")

	require.NoError(t, m.Create(ctx, createOpts))
	bogus := entity.PermissionSetOptions{Permissions: []string{rbac.PermissionUserRead, "no_such_permission"}}
	err = m.SetPermissions(ctx, testRoleID, bogus)
	assert.True(t, isBadRequest(err), "setting an undefined permission: got %v", err)
	err = m.SetPermissionsForRoleName(ctx, createOpts.Name, bogus)
	assert.True(t, isBadRequest(err), "setting an undefined permission by name: got %v", err)
	perms, err := m.GetPermissions(ctx, testRoleID)
	require.NoError(t, err)
	assert.Empty(t, perms, "a refused permission list is not partly stored")
}
