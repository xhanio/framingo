package role

import (
	"context"
	"testing"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/types/model"
	"github.com/xhanio/framingo/pkg/utils/testutil"

	"github.com/xhanio/framingo/example/pkg/services/repository"
	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/rbac"
)

var (
	createOpts = entity.RoleCreateOptions{
		Name:        "test",
		Description: "Test Role",
	}

	testRoleID     = int32(4)
	notExistRoleID = int32(10)
)

// The following tests using actual test database
func setup() (*manager, model.Database, error) {
	db, err := testutil.SetupDB()
	if err != nil {
		return nil, nil, errors.Wrap(err)
	}
	m := newRole(repository.New(db))
	if err := m.Init(context.Background()); err != nil {
		return nil, nil, errors.Wrap(err)
	}
	return m, db, nil
}

func TestCreateList(t *testing.T) {
	m, db, err := setup()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		// Clean up database
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
		// Check new user information
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
		// Clean up database
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
		// Clean up database
		if err := db.Cleanup(true); err != nil {
			t.Fatalf("Error cleaning up dbManager: %v", err)
		}
	}()

	// The role record to be deleted does not exist.
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
		// Clean up database
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
		// Clean up database
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
		// Clean up database
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
