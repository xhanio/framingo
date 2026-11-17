package user

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/model"
	"github.com/xhanio/framingo/pkg/utils/testutil"

	"github.com/xhanio/framingo/example/pkg/services/repository"
	"github.com/xhanio/framingo/example/pkg/services/system/platform"
	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/message"
	"github.com/xhanio/framingo/example/pkg/types/orm"
	"github.com/xhanio/framingo/example/pkg/types/preset"
	"github.com/xhanio/framingo/example/pkg/types/rbac"
)

var (
	testUser    = "johndoe"
	testUserIDs = []int32{2, 3}

	createOpts = entity.UserCreateOptions{
		OrganizationID: preset.DefaultOrganizationID,
		Username:       testUser,
		FirstName:      "John",
		LastName:       "Doe",
		Email:          "johndoe@example.com",
		Password:       "password123",
		Role:           "admin",
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

// The following tests using actual test database. setup hands back the
// database's debug side, which the tests clean up through; the service only
// ever sees model.Database.
func setup() (*manager, model.DatabaseDebugger, error) {
	return setupWithBus(nil)
}

// setupWithBus is setup with the messagebus the platform carries, where the
// service publishes its account changes.
func setupWithBus(mb model.MessageBus) (*manager, model.DatabaseDebugger, error) {
	db, err := testutil.SetupDB()
	if err != nil {
		return nil, nil, errors.Wrap(err)
	}
	if err := db.ORM().AutoMigrate(
		&orm.Organization{},
		&orm.Role{},
		&orm.RolePermission{},
		&orm.Contact{},
		&orm.User{},
		&orm.Certificate{},
	); err != nil {
		return nil, nil, errors.Wrap(err)
	}
	m := newManager(platform.New(db, nil, nil, mb), repository.New(platform.New(db, nil, nil, nil)))
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

	// An unsortable column is rejected by the options validator before it can
	// reach the database. It used to surface as DBFailed — a 500 that also
	// told the caller whether the column existed; now it is a 400.
	if _, err := m.List(context.Background(), listFalseSortOpts); err != nil {
		if be, ok := err.(errors.Error); ok {
			if !errors.Is(be, errors.InvalidArgument) {
				t.Fatalf("Expected %v for a non-sortable column, but got: %v", errors.InvalidArgument, err.Error())
			}
		}
	} else {
		t.Fatalf("Expecting error from ordering by a non-sortable column")
	}

	// A column that exists but must not be exposed for ordering.
	if _, err := m.List(context.Background(), entity.UserListOptions{SortBy: "password"}); err == nil {
		t.Fatal("ordering by password should be rejected")
	}

	// The allowed columns still work.
	for _, col := range []string{"", "id", "username", "role"} {
		if _, err := m.List(context.Background(), entity.UserListOptions{SortBy: col}); err != nil {
			t.Fatalf("sort_by=%q should be accepted, got %v", col, err)
		}
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
		var updated *entity.User
		for _, u := range users {
			if u.ID == 2 {
				updated = u
				break
			}
		}
		if updated == nil {
			t.Errorf("Updated user with ID 2 not found in list")
		} else if updated.Username != *updateOpts.Username {
			t.Errorf("Updated username does not match expected values.")
		}
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

// recorder is a messagebus that keeps what it is sent. SendMessage is the one
// method the service drives; the rest is left to the embedded nil interface,
// so a call to it panics as a test bug.
type recorder struct {
	model.MessageBus
	msgs []common.Message
}

func (r *recorder) Name() string                   { return "test/recorder" }
func (r *recorder) Dependencies() []common.Service { return nil }
func (r *recorder) SendMessage(_ context.Context, _ common.Named, msg common.Message) {
	r.msgs = append(r.msgs, msg)
}

// Every committed change that invalidates a session reaches the sender, by
// user ID; a change that leaves the credential intact sends nothing.
func TestAccountChangesArePublished(t *testing.T) {
	sent := &recorder{}
	m, db, err := setupWithBus(sent)
	require.NoError(t, err)
	defer func() { require.NoError(t, db.Cleanup(true)) }()
	ctx := context.Background()

	_, err = m.Create(ctx, createOpts) // ID 2
	require.NoError(t, err)

	_, err = m.Update(ctx, 2, entity.UserUpdateOptions{FirstName: stringPointer("Jon")})
	require.NoError(t, err)
	assert.Empty(t, sent.msgs, "a contact-only update leaves the credential intact")

	_, err = m.Update(ctx, 2, entity.UserUpdateOptions{Role: stringPointer(rbac.RoleUser)})
	require.NoError(t, err)
	require.NoError(t, m.ResetPassword(ctx, false, 2, entity.UserResetPasswordOptions{Password: "password_new"}))
	require.NoError(t, m.Delete(ctx, []int32{2}))
	assert.Equal(t, []common.Message{
		message.LocalUserIdentityChanged{UserID: 2},
		message.LocalUserPasswordReset{UserID: 2},
		message.LocalUsersDeleted{UserIDs: []int32{2}},
	}, sent.msgs)

	// A refused change publishes nothing.
	sent.msgs = nil
	require.Error(t, m.Delete(ctx, []int32{preset.AdminUserID}))
	assert.Empty(t, sent.msgs)
}

// A user created without an organization - what the HTTP API sends - joins
// the default one, so it can log in. It used to land in organization 0, where
// no login ever looks.
func TestCreateDefaultsToDefaultOrganization(t *testing.T) {
	m, db, err := setup()
	require.NoError(t, err)
	defer func() { require.NoError(t, db.Cleanup(true)) }()
	ctx := context.Background()

	opts := createOpts2 // no OrganizationID
	require.Zero(t, opts.OrganizationID)
	user, err := m.Create(ctx, opts)
	require.NoError(t, err)
	assert.Equal(t, preset.DefaultOrganizationID, user.OrganizationID)

	_, err = m.Authenticate(ctx, preset.DefaultOrganizationName, opts.Username, opts.Password)
	assert.NoError(t, err, "the new user can log in")
}

// An edit leaves every field it does not name alone - the password-change
// flag included, which a non-pointer field cleared on every edit.
func TestUpdateKeepsPasswordChangeFlag(t *testing.T) {
	m, db, err := setup()
	require.NoError(t, err)
	defer func() { require.NoError(t, db.Cleanup(true)) }()
	ctx := context.Background()

	opts := createOpts
	opts.ChangePassword = true
	_, err = m.Create(ctx, opts) // ID 2
	require.NoError(t, err)

	user, err := m.Update(ctx, 2, entity.UserUpdateOptions{FirstName: stringPointer("Johnny")})
	require.NoError(t, err)
	assert.True(t, user.ChangePassword, "an edit that does not name the flag must keep it")

	off := false
	user, err = m.Update(ctx, 2, entity.UserUpdateOptions{Expired: &off})
	require.NoError(t, err)
	assert.False(t, user.ChangePassword, "an edit that names the flag sets it")
}
