package user

import (
	"context"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/utils/sliceutil"
	"golang.org/x/crypto/bcrypt"

	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/message"
	"github.com/xhanio/framingo/example/pkg/types/preset"
	"github.com/xhanio/framingo/example/pkg/types/rbac"
)

// POST /users/local-users
// Create new user for all given organizations.
func (m *manager) Create(ctx context.Context, opts entity.UserCreateOptions) (*entity.User, error) {
	if opts.Username == "" || opts.Password == "" {
		return nil, errors.InvalidArgument.Newf("Invalid Arguments: Username, Password cannot be empty")
	}
	user, contact, err := m.repository.CreateUser(ctx, opts)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return toEntity(user, contact), nil
}

// GET /users/local-users
// TODO: Support multiple sort field and filtering.
func (m *manager) List(ctx context.Context, opts entity.UserListOptions) ([]*entity.User, error) {
	users, contacts, err := m.repository.ListUsers(ctx, opts)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	if len(users) != len(contacts) {
		return nil, errors.Newf("failed to list users: contact number mismatch")
	}
	var results []*entity.User
	for i, user := range users {
		results = append(results, toEntity(user, contacts[i]))
	}
	return results, nil
}

// DEL /users/local-users
func (m *manager) Delete(ctx context.Context, userIDs []int32) error {
	if sliceutil.In(preset.AdminUserID, userIDs...) {
		return errors.Forbidden.Newf("Cannot delete system built-in default admin user")
	}
	msg := message.DeleteLocalUsers{Usernames: []string{}}
	err := m.repository.Transaction(ctx, func(tctx context.Context) error {
		for _, userID := range userIDs {
			user, err := m.Get(tctx, userID)
			if err != nil {
				return errors.Wrap(err)
			}
			if err := m.repository.DeleteUser(tctx, userID); err != nil {
				return errors.Wrap(err)
			}
			msg.Usernames = append(msg.Usernames, user.Username)
		}
		return nil
	})
	if err != nil {
		return errors.Wrap(err)
	}
	m.sender.SendMessage(ctx, m, msg)
	return nil
}

// GET /users/local-users/{userID}
func (m *manager) Get(ctx context.Context, userID int32) (*entity.User, error) {
	user, contact, err := m.repository.GetUser(ctx, userID)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return toEntity(user, contact), nil
}

// PATCH /users/local-users/{userID}
func (m *manager) Update(ctx context.Context, userID int32, opts entity.UserUpdateOptions) (*entity.User, error) {
	if opts.Username != nil && *opts.Username == "" {
		return nil, errors.InvalidArgument.Newf("Invalid Arguments: Username cannot be empty")
	}
	if userID == preset.AdminUserID && opts.Role != nil && *opts.Role != rbac.RoleAdmin {
		return nil, errors.Forbidden.Newf("Cannot update the built-in admin role to non-admin.")
	}
	user, contact, err := m.repository.UpdateUser(ctx, userID, opts)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return toEntity(user, contact), nil
}

// PUT /users/local-users/{userID}/reset-password
// Admins can reset password for any other users without validating the password.
func (m *manager) ResetPassword(ctx context.Context, isResetOwnPwd bool, userID int32, opts entity.UserResetPasswordOptions) error {
	if isResetOwnPwd && opts.OldPassword == "" {
		return errors.InvalidArgument.Newf("Invalid Arguments: Old password length should not be empty")
	}
	msg := message.ResetLocalUserPassword{}
	err := m.repository.Transaction(ctx, func(tctx context.Context) error {
		user, _, err := m.repository.GetUser(tctx, userID)
		if err != nil {
			return errors.Wrap(err)
		}
		if isResetOwnPwd && bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(opts.OldPassword)) != nil {
			return errors.Forbidden.Newf("Access Denied: Old password validation failed.")
		}
		msg.Username = user.Username
		return m.repository.ResetUserPassword(tctx, userID, opts.Password)
	})
	if err != nil {
		return errors.Wrapf(err, "failed to reset user password")
	}
	// Send event to auth service to logout the user's active sessions
	m.sender.SendMessage(ctx, m, msg)
	return nil
}

// // GET /get-sp-metadata
// func (m *manager) GetSAMLMetadata(ctx context.Context) error {
// 	return nil
// }

// // PUT /update-saml-settings
// func (m *manager) UpdateSAMLSettings(ctx context.Context) error {
// 	return nil
// }

// // DEL /delete-saml-settings
// func (m *manager) DeleteSAMLSettings(ctx context.Context) error {
// 	return nil
// }

// Validate the existance of the user with given username.
func (m *manager) Validate(ctx context.Context, organization, username string) (bool, error) {
	organizationID, err := m.repository.GetOrganizationID(ctx, organization)
	if err != nil {
		return false, errors.Unauthorized.Newf("Failed to validate username")
	}
	if _, err := m.repository.GetUserByName(ctx, organizationID, username); err != nil {
		return false, errors.Unauthorized.Newf("Failed to validate username")
	}
	return true, nil
}

func (m *manager) Authenticate(ctx context.Context, organization, username, password string) (*entity.Credential, error) {
	organizationID, err := m.repository.GetOrganizationID(ctx, organization)
	if err != nil {
		return nil, errors.Unauthorized.Newf("Failed to auth user %s, username & password pair does not match any record", username)
	}
	user, err := m.repository.GetUserByName(ctx, organizationID, username)
	if err != nil {
		return nil, errors.Unauthorized.Newf("Failed to auth user %s, username & password pair does not match any record", username)
	}
	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) != nil {
		return nil, errors.Unauthorized.Newf("Failed to auth user %s, username & password pair does not match any record", username)
	}
	return &entity.Credential{
		Source:               preset.AuthSourceLocalUser,
		Role:                 user.Role,
		UserID:               user.ID,
		UserName:             user.Username,
		RequirePasswordReset: user.RequirePasswordReset,
		OrganizationID:       organizationID,
		OrganizationName:     organization,
	}, nil
}
