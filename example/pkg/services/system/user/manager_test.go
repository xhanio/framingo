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

// the seeded admin is user 1, so these get IDs 2 and 3 when created in order
var (
	createOpts = entity.UserCreateOptions{
		OrganizationID: preset.DefaultOrganizationID,
		Username:       "johndoe",
		FirstName:      "John",
		LastName:       "Doe",
		Email:          "johndoe@example.com",
		Password:       "password123",
		Role:           rbac.RoleAdmin,
	}
	createOpts2 = entity.UserCreateOptions{
		Username:  "AAAA",
		FirstName: "A",
		LastName:  "AAA",
		Email:     "AAAA@example.com",
		Password:  "password123",
		Role:      rbac.RoleAdmin,
	}
)

func stringPointer(s string) *string {
	return &s
}

func isA(category errors.Category) func(error) bool {
	return func(err error) bool { return err != nil && errors.Is(err, category) }
}

// setup returns an initialized service publishing on mb, which may be nil.
func setup(t *testing.T, mb model.MessageBus) *manager {
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
	r := repository.New(platform.New(db, nil, nil, nil))
	m := newManager(platform.New(db, nil, nil, mb), r)
	require.NoError(t, m.Init(context.Background()))
	return m
}

func TestCreateAndList(t *testing.T) {
	m := setup(t, nil)
	ctx := context.Background()
	byName := entity.UserListOptions{SortBy: "username"}

	users, err := m.List(ctx, byName)
	require.NoError(t, err)
	assert.Len(t, users, 1, "the seeded admin")

	user, err := m.Create(ctx, createOpts)
	require.NoError(t, err)
	assert.Equal(t, int32(2), user.ID)
	_, err = m.Create(ctx, createOpts2)
	require.NoError(t, err)

	users, err = m.List(ctx, byName)
	require.NoError(t, err)
	require.Len(t, users, 3)
	assert.Equal(t, createOpts2.Username, users[0].Username, "sorted by username")
}

func TestListSortColumns(t *testing.T) {
	m := setup(t, nil)
	ctx := context.Background()

	for _, col := range []string{"notexist", "password"} {
		_, err := m.List(ctx, entity.UserListOptions{SortBy: col})
		assert.True(t, isA(errors.InvalidArgument)(err), "sort_by=%q: got %v", col, err)
	}
	for _, col := range []string{"", "id", "username", "role"} {
		_, err := m.List(ctx, entity.UserListOptions{SortBy: col})
		assert.NoError(t, err, "sort_by=%q", col)
	}
}

func TestGet(t *testing.T) {
	m := setup(t, nil)
	ctx := context.Background()
	_, err := m.Create(ctx, createOpts)
	require.NoError(t, err)

	user, err := m.Get(ctx, 2)
	require.NoError(t, err)
	assert.Equal(t, createOpts.Username, user.Username)

	_, err = m.Get(ctx, 5)
	assert.True(t, isA(errors.NotFound)(err), "an unknown user: got %v", err)
}

func TestUpdate(t *testing.T) {
	m := setup(t, nil)
	ctx := context.Background()
	_, err := m.Create(ctx, createOpts)
	require.NoError(t, err)

	opts := entity.UserUpdateOptions{
		Username:  stringPointer("Bobdoe"),
		Role:      stringPointer(rbac.RoleUser),
		FirstName: stringPointer("Bob"),
		LastName:  stringPointer("Doe"),
		Email:     stringPointer(""),
	}
	_, err = m.Update(ctx, 2, opts)
	require.NoError(t, err)
	user, err := m.Get(ctx, 2)
	require.NoError(t, err)
	assert.Equal(t, "Bobdoe", user.Username)
	assert.Equal(t, rbac.RoleUser, user.Role)
	assert.Equal(t, "Bob", user.FirstName)
	assert.Equal(t, "Doe", user.LastName)
	assert.Empty(t, user.Email, "a field set to empty is written")

	_, err = m.Update(ctx, 5, opts)
	assert.True(t, isA(errors.NotFound)(err), "an unknown user: got %v", err)
}

func TestUpdateKeepsPasswordChangeFlag(t *testing.T) {
	m := setup(t, nil)
	ctx := context.Background()
	opts := createOpts
	opts.ChangePassword = true
	_, err := m.Create(ctx, opts) // ID 2
	require.NoError(t, err)

	user, err := m.Update(ctx, 2, entity.UserUpdateOptions{FirstName: stringPointer("Johnny")})
	require.NoError(t, err)
	assert.True(t, user.ChangePassword, "an edit that does not name the flag keeps it")

	off := false
	user, err = m.Update(ctx, 2, entity.UserUpdateOptions{Expired: &off})
	require.NoError(t, err)
	assert.False(t, user.ChangePassword, "an edit that names the flag sets it")
}

func TestDelete(t *testing.T) {
	m := setup(t, nil)
	ctx := context.Background()

	err := m.Delete(ctx, []int32{2, 3})
	assert.True(t, isA(errors.NotFound)(err), "unknown users: got %v", err)

	_, err = m.Create(ctx, createOpts)
	require.NoError(t, err)
	_, err = m.Create(ctx, createOpts2)
	require.NoError(t, err)
	require.NoError(t, m.Delete(ctx, []int32{2, 3}))
	users, err := m.List(ctx, entity.UserListOptions{})
	require.NoError(t, err)
	assert.Len(t, users, 1, "only the seeded admin is left")
}

// The built-in admin is neither deleted nor demoted, whoever calls; its
// contact details stay editable.
func TestInvariants(t *testing.T) {
	m := setup(t, nil)
	ctx := context.Background()
	forbidden := isA(errors.Forbidden)

	err := m.Delete(ctx, []int32{preset.AdminUserID})
	assert.True(t, forbidden(err), "delete the built-in admin: %v", err)
	_, err = m.Update(ctx, preset.AdminUserID, entity.UserUpdateOptions{Role: stringPointer(rbac.RoleUser)})
	assert.True(t, forbidden(err), "demote the built-in admin: %v", err)
	_, err = m.Update(ctx, preset.AdminUserID, entity.UserUpdateOptions{FirstName: stringPointer("Ada")})
	assert.NoError(t, err, "the built-in admin's contact details are editable")
}

func TestCreateDefaultsToDefaultOrganization(t *testing.T) {
	m := setup(t, nil)
	ctx := context.Background()
	opts := createOpts2
	require.Zero(t, opts.OrganizationID)

	user, err := m.Create(ctx, opts)
	require.NoError(t, err)
	assert.Equal(t, preset.DefaultOrganizationID, user.OrganizationID)
	_, err = m.Authenticate(ctx, preset.DefaultOrganizationName, opts.Username, opts.Password)
	assert.NoError(t, err, "the new user can log in")
}

func TestAuthenticate(t *testing.T) {
	m := setup(t, nil)
	ctx := context.Background()
	_, err := m.Create(ctx, createOpts)
	require.NoError(t, err)

	credential, err := m.Authenticate(ctx, preset.DefaultOrganizationName, createOpts.Username, createOpts.Password)
	require.NoError(t, err)
	assert.Equal(t, int32(2), credential.UserID)
	assert.Equal(t, createOpts.Username, credential.UserName)

	_, err = m.Authenticate(ctx, preset.DefaultOrganizationName, createOpts.Username, "wrong")
	assert.True(t, isA(errors.Unauthorized)(err), "a wrong password: got %v", err)
	_, err = m.Authenticate(ctx, "no-such-org", createOpts.Username, createOpts.Password)
	assert.True(t, isA(errors.Unauthorized)(err), "an unknown organization: got %v", err)
}

func TestValidate(t *testing.T) {
	m := setup(t, nil)
	ctx := context.Background()
	_, err := m.Create(ctx, createOpts)
	require.NoError(t, err)

	ok, err := m.Validate(ctx, preset.DefaultOrganizationName, createOpts.Username)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestResetPassword(t *testing.T) {
	m := setup(t, nil)
	ctx := context.Background()
	_, err := m.Create(ctx, createOpts)
	require.NoError(t, err)

	require.NoError(t, m.ResetPassword(ctx, 2, entity.UserResetPasswordOptions{Password: "password_new"}))
	_, err = m.Authenticate(ctx, preset.DefaultOrganizationName, createOpts.Username, "password_new")
	assert.NoError(t, err, "the reset password logs in, no old one needed")

	err = m.ResetPassword(ctx, 5, entity.UserResetPasswordOptions{Password: "password_new"})
	assert.True(t, isA(errors.NotFound)(err), "an unknown user: got %v", err)
}

func TestChangePassword(t *testing.T) {
	m := setup(t, nil)
	ctx := context.Background()
	_, err := m.Create(ctx, createOpts) // ID 2
	require.NoError(t, err)

	err = m.ChangePassword(ctx, 2, entity.UserChangePasswordOptions{OldPassword: "wrong", Password: "password_new"})
	assert.True(t, isA(errors.Forbidden)(err), "a wrong old password: got %v", err)
	err = m.ChangePassword(ctx, 2, entity.UserChangePasswordOptions{Password: "password_new"})
	assert.True(t, isA(errors.InvalidArgument)(err), "no old password: got %v", err)
	_, err = m.Authenticate(ctx, preset.DefaultOrganizationName, createOpts.Username, createOpts.Password)
	require.NoError(t, err, "a refused change leaves the password")

	require.NoError(t, m.ChangePassword(ctx, 2, entity.UserChangePasswordOptions{OldPassword: createOpts.Password, Password: "password_new"}))
	_, err = m.Authenticate(ctx, preset.DefaultOrganizationName, createOpts.Username, "password_new")
	require.NoError(t, err, "the changed password logs in")
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
	m := setup(t, sent)
	ctx := context.Background()
	_, err := m.Create(ctx, createOpts) // ID 2
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

	sent.msgs = nil
	require.Error(t, m.Delete(ctx, []int32{preset.AdminUserID}))
	assert.Empty(t, sent.msgs, "a refused change publishes nothing")
}
