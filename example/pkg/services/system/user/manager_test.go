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
		Password: "password_new",
	}
)

func stringPointer(s string) *string {
	return &s
}

func setup() (*manager, model.DatabaseDebugger, error) {
	return setupWithBus(nil)
}

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
	r := repository.New(platform.New(db, nil, nil, nil))
	m := newManager(platform.New(db, nil, nil, mb), r)
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

	if _, err := m.Create(context.Background(), createOpts2); err != nil {
		t.Fatalf("Error from Create User: %v", err.Error())
	}

	if users, err := m.List(context.Background(), listOpts); err != nil {
		t.Fatalf("Error from List User: %v", err.Error())
	} else if len(users) != 3 {
		t.Errorf("Expected number of users: got = %v; expected %v", len(users), 3)
	} else {
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
			if !errors.Is(be, errors.InvalidArgument) {
				t.Fatalf("Expected %v for a non-sortable column, but got: %v", errors.InvalidArgument, err.Error())
			}
		}
	} else {
		t.Fatalf("Expecting error from ordering by a non-sortable column")
	}

	if _, err := m.List(context.Background(), entity.UserListOptions{SortBy: "password"}); err == nil {
		t.Fatal("ordering by password should be rejected")
	}

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
		if err := db.Cleanup(true); err != nil {
			t.Fatalf("Error cleaning up dbManager: %v", err)
		}
	}()

	if err := m.Delete(context.Background(), testUserIDs); err != nil {
		if be, ok := err.(errors.Error); ok {
			if !errors.Is(be, errors.NotFound) {
				t.Fatalf("Expected error from Delete User: %v, but got: %v", errors.NotFound, err.Error())
			}
		}
	} else {
		t.Fatalf("Expecting error from deleting not existing user")
	}

	if _, err := m.Create(context.Background(), createOpts); err != nil {
		t.Fatalf("Error from Create User: %v", err.Error())
	}
	if _, err := m.Create(context.Background(), createOpts2); err != nil {
		t.Fatalf("Error from Create User: %v", err.Error())
	}

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
		if err := db.Cleanup(true); err != nil {
			t.Fatalf("Error cleaning up dbManager: %v", err)
		}
	}()

	if _, err := m.Create(context.Background(), createOpts); err != nil {
		t.Fatalf("Error from Create User: %v", err.Error())
	}

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
		if err := db.Cleanup(true); err != nil {
			t.Fatalf("Error cleaning up dbManager: %v", err)
		}
	}()

	if _, err := m.Create(context.Background(), createOpts); err != nil {
		t.Fatalf("Error from Create User: %v", err.Error())
	}

	if err := m.ResetPassword(context.Background(), 2, resetPasswordOpts); err != nil {
		t.Fatalf("Error from ResetPassword User: %v", err.Error())
	}

	if err := m.ResetPassword(context.Background(), 5, resetPasswordOpts); err != nil {
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

// unimplemented methods hit the embedded nil interface and panic
type recorder struct {
	model.MessageBus
	msgs []common.Message
}

func (r *recorder) Name() string                   { return "test/recorder" }
func (r *recorder) Dependencies() []common.Service { return nil }
func (r *recorder) SendMessage(_ context.Context, _ common.Named, msg common.Message) {
	r.msgs = append(r.msgs, msg)
}

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
	require.NoError(t, m.ResetPassword(ctx, 2, entity.UserResetPasswordOptions{Password: "password_new"}))
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

func TestChangePassword(t *testing.T) {
	m, db, err := setup()
	require.NoError(t, err)
	defer func() { require.NoError(t, db.Cleanup(true)) }()
	ctx := context.Background()
	_, err = m.Create(ctx, createOpts) // ID 2
	require.NoError(t, err)

	err = m.ChangePassword(ctx, 2, entity.UserChangePasswordOptions{OldPassword: "wrong", Password: "password_new"})
	assert.True(t, errors.Is(err, errors.Forbidden), "a wrong old password: got %v", err)
	err = m.ChangePassword(ctx, 2, entity.UserChangePasswordOptions{Password: "password_new"})
	assert.True(t, errors.Is(err, errors.InvalidArgument), "no old password: got %v", err)
	_, err = m.Authenticate(ctx, preset.DefaultOrganizationName, createOpts.Username, createOpts.Password)
	require.NoError(t, err, "a refused change leaves the password")

	require.NoError(t, m.ChangePassword(ctx, 2, entity.UserChangePasswordOptions{OldPassword: createOpts.Password, Password: "password_new"}))
	_, err = m.Authenticate(ctx, preset.DefaultOrganizationName, createOpts.Username, "password_new")
	require.NoError(t, err, "the changed password logs in")

	require.NoError(t, m.ResetPassword(ctx, 2, entity.UserResetPasswordOptions{Password: "password_reset"}))
	_, err = m.Authenticate(ctx, preset.DefaultOrganizationName, createOpts.Username, "password_reset")
	require.NoError(t, err, "a reset needs no old password")
}
