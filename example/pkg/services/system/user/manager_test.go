package user

import (
	"context"
	"testing"
	"time"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/types/model"
	"github.com/xhanio/framingo/pkg/utils/testutil"

	"github.com/xhanio/framingo/example/pkg/services/repository"
	"github.com/xhanio/framingo/example/pkg/types/entity"
)

var usersTable = map[string]interface{}{
	"ID":                1,
	"Erased":            false,
	"Hidden":            false,
	"UpdateTime":        time.Time{},
	"Version":           int64(0),
	"Expired":           false,
	"Locked":            false,
	"PassCanExpired":    false,
	"Disabled":          false,
	"FailedLoginsCount": 0,
	"Password":          "",
	"Username":          "",
	"Role":              "",
	"OrganizationID":    0,
	"ContactID":         nil,
}

var (
	testUser    = "johndoe"
	testUserIDs = []int32{1, 2}

	createOpts = entity.UserCreateOptions{
		Username:  testUser,
		FirstName: "John",
		LastName:  "Doe",
		Email:     "johndoe@example.com",
		Password:  "password123",
		Role:      "admin",
	}

	createOpts2 = entity.UserCreateOptions{
		Username:  "AAAA",
		FirstName: "A",
		LastName:  "AAA",
		Email:     "AAAA@example.com",
		Password:  "password123",
		Role:      "admin",
	}

	listOpts          = entity.UserListOptions{SortBy: "username"}
	listFalseSortOpts = entity.UserListOptions{SortBy: "notexist"}

	updateOpts = entity.UserUpdateOptions{
		Username:  stringPointer("Bobdoe"),
		Role:      stringPointer("user"),
		FirstName: stringPointer("Bob"),
		Email:     stringPointer(""),
		LastName:  stringPointer("Doe"),
	}

	resetPasswordOpts = entity.UserResetPasswordOptions{
		Password:    "password_new",
		OldPassword: "password123",
	}
)

func stringPointer(s string) *string {
	return &s
}

// The following tests using actual test database
func setup() (*manager, model.Database, error) {
	db, err := testutil.SetupDB()
	if err != nil {
		return nil, nil, errors.Wrap(err)
	}
	m := newUser(repository.New(db))
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

	if users, err := m.List(context.Background(), listOpts); err != nil {
		t.Fatalf("Error from List User: %v", err.Error())
	} else if len(users) != 1 {
		t.Errorf("Expected number of users: got = %v; expected %v", len(users), 1)
	}

	res, err := m.Create(context.Background(), createOpts)
	if err != nil {
		t.Fatalf("Error from Create User: %v", err.Error())
	}
	if res.ID != 2 {
		t.Fatalf("ID is not correct: got = %v; expected %v", res.ID, 2)
	}

	if users, err := m.List(context.Background(), listOpts); err != nil {
		t.Fatalf("Error from List User: %v", err.Error())
	} else if len(users) != 2 {
		t.Errorf("Expected number of users: got = %v; expected %v", len(users), 2)
	}

	// Add another user
	if _, err := m.Create(context.Background(), createOpts2); err != nil {
		t.Fatalf("Error from Create User: %v", err.Error())
	}

	if users, err := m.List(context.Background(), listOpts); err != nil {
		t.Fatalf("Error from List User: %v", err.Error())
	} else if len(users) != 3 {
		t.Errorf("Expected number of users: got = %v; expected %v", len(users), 3)
	} else {
		// Check new user information
		firstUser := users[0]
		if firstUser.Username != createOpts2.Username {
			t.Errorf("Failed to sort users by username: got = %v; expected %v", firstUser.Username, createOpts2.Username)
		}
	}
}

func TestListWithSort(t *testing.T) {
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

	if _, err := m.Create(context.Background(), createOpts); err != nil {
		t.Fatalf("Error from Create User: %v", err.Error())
	}
	if _, err := m.Create(context.Background(), createOpts2); err != nil {
		t.Fatalf("Error from Create User: %v", err.Error())
	}

	if _, err := m.List(context.Background(), listFalseSortOpts); err != nil {
		if be, ok := err.(errors.Error); ok {
			if !errors.Is(be, errors.DBFailed) {
				t.Fatalf("Expected error from ordering by a non-existing column: %v, but got: %v", errors.DBFailed, err.Error())
			}
		}
	} else {
		t.Fatalf("Expecting error from listing not existing user")
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

	// The user record to be deleted does not exist.
	if err := m.Delete(context.Background(), testUserIDs); err != nil {
		if be, ok := err.(errors.Error); ok {
			if !errors.Is(be, errors.NotFound) {
				t.Fatalf("Expected error from Delete User: %v, but got: %v", errors.NotFound, err.Error())
			}
		}
	} else {
		t.Fatalf("Expecting error from deleting not existing user")
	}

	// Create two users with same create options.
	if _, err := m.Create(context.Background(), createOpts); err != nil {
		t.Fatalf("Error from Create User: %v", err.Error())
	}
	if _, err := m.Create(context.Background(), createOpts2); err != nil {
		t.Fatalf("Error from Create User: %v", err.Error())
	}

	// Delete the users.
	if err := m.Delete(context.Background(), testUserIDs); err != nil {
		t.Fatalf("Error from Delete User: %v", err.Error())
	}

	if users, err := m.List(context.Background(), listOpts); err != nil {
		t.Fatalf("Error from List User: %v", err.Error())
	} else if len(users) != 1 {
		t.Errorf("Expected number of users: got = %v; expected = %v", len(users), 1)
	}
}

func TestGet(t *testing.T) {
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

	if _, err := m.Create(context.Background(), createOpts); err != nil {
		t.Fatalf("Error from Create User: %v", err.Error())
	}

	if user, err := m.Get(context.Background(), 2); err != nil {
		t.Fatalf("Error from List User: %v", err.Error())
	} else if user.Username != createOpts.Username {
		t.Errorf("Expected get target user with username = %v; got %v", user.Username, createOpts.Username)
	}

	// List not existing user
	if _, err := m.Get(context.Background(), 5); err != nil {
		if be, ok := err.(errors.Error); ok {
			if !errors.Is(be, errors.NotFound) {
				t.Fatalf("Expected error from listing user by ID: %v, but got: %v", errors.DBFailed, err.Error())
			}
		}
	} else {
		t.Fatalf("Expecting error from listing not existing user")
	}
}

func TestUpdate(t *testing.T) {
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

	if _, err := m.Create(context.Background(), createOpts); err != nil {
		t.Fatalf("Error from Create User: %v", err.Error())
	}

	// Update user with ID = 2
	if _, err := m.Update(context.Background(), 2, updateOpts); err != nil {
		t.Fatalf("Error from Update User: %v", err.Error())
	}

	if users, err := m.List(context.Background(), listOpts); err != nil {
		t.Fatalf("Error from List User: %v", err.Error())
	} else if len(users) != 2 {
		t.Errorf("Expected number of users: got = %v; expected %v", len(users), 2)
	} else {
		user := users[1]
		if user.Username != *updateOpts.Username {
			t.Errorf("Updated username does not match expected values.")
		}
		// if user.Role != *updateOpts.Role {
		// 	t.Errorf("Updated role does not match expected values.")
		// }
	}

	if user, err := m.Get(context.Background(), 2); err != nil {
		t.Fatalf("Error from List user contact info: %v", err.Error())
	} else {
		if user.FirstName != *updateOpts.FirstName {
			t.Errorf("Updated Firstname does not match expected values.")
		}
		if user.LastName != *updateOpts.LastName {
			t.Errorf("Updated LastName does not match expected values.")
		}
		if user.Email != *updateOpts.Email {
			t.Errorf("Updated Email does not match expected values.")
		}
	}

	// Update not existing user
	if _, err := m.Update(context.Background(), 5, updateOpts); err != nil {
		if be, ok := err.(errors.Error); ok {
			if !errors.Is(be, errors.NotFound) {
				t.Fatalf("Expected error from updating user by ID: %v, but got: %v", errors.DBFailed, err.Error())
			}
		}
	} else {
		t.Fatalf("Expecting error from updating not existing user")
	}
}

func TestResetPassword(t *testing.T) {
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

	if _, err := m.Create(context.Background(), createOpts); err != nil {
		t.Fatalf("Error from Create User: %v", err.Error())
	}

	if err := m.ResetPassword(context.Background(), false, 2, resetPasswordOpts); err != nil {
		t.Fatalf("Error from ResetPassword User: %v", err.Error())
	}

	// Reset password for not existing user
	if err := m.ResetPassword(context.Background(), false, 5, resetPasswordOpts); err != nil {
		if be, ok := err.(errors.Error); ok {
			if !errors.Is(be, errors.NotFound) {
				t.Fatalf("Expected error from updating user by ID: %v, but got: %v", errors.DBFailed, err.Error())
			}
		}
	} else {
		t.Fatalf("Expecting error from resetting password for not existing user")
	}
}

func TestValidate(t *testing.T) {
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

	if _, err := m.Create(context.Background(), createOpts); err != nil {
		t.Fatalf("Error from Create User: %v", err.Error())
	}

	if res, err := m.Validate(context.Background(), "default", testUser); err != nil {
		t.Fatalf("Error from validating User: %v", err.Error())
	} else if !res {
		t.Errorf("Failed to validate existing user")
	}

}

func TestAuthenticate(t *testing.T) {
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

	if _, err := m.Create(context.Background(), createOpts); err != nil {
		t.Fatalf("Error from Create User: %v", err.Error())
	}

	if credential, err := m.Authenticate(context.Background(), "default", testUser, "password123"); err != nil {
		t.Fatalf("Error from authenticating User: %v", err.Error())
	} else if credential.UserID != 2 || credential.UserName != testUser {
		t.Errorf("Failed to authenticate existing user")
	}
}
