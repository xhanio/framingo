package user

import (
	"context"

	"github.com/go-playground/validator/v10"
	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/utils/sliceutil"

	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/message"
	"github.com/xhanio/framingo/example/pkg/types/preset"
	"github.com/xhanio/framingo/example/pkg/types/rbac"
	"github.com/xhanio/framingo/example/pkg/types/repo"
)

// concurrency-safe and caches struct metadata, so built once
var validate = validator.New()

// accounts are acted on by ID without asking who calls: route permissions
// decide the caller; the rules here hold for every caller

func (m *manager) Create(ctx context.Context, opts entity.UserCreateOptions) (*entity.User, error) {
	if opts.Username == "" || opts.Password == "" {
		return nil, errors.InvalidArgument.Newf("Invalid Arguments: Username, Password cannot be empty")
	}
	// zero would land in an organization login never looks in
	if opts.OrganizationID == 0 {
		opts.OrganizationID = preset.DefaultOrganizationID
	}
	hash, err := hashPassword(opts.Password)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	// the repository stores the hash, never the password
	user, err := m.repository.CreateUser(ctx, repo.UserCreateOptions{
		OrganizationID:       opts.OrganizationID,
		Username:             opts.Username,
		PasswordHash:         hash,
		Role:                 opts.Role,
		FirstName:            opts.FirstName,
		LastName:             opts.LastName,
		Email:                opts.Email,
		Title:                opts.Title,
		ChangePassword:       opts.ChangePassword,
		RequirePasswordReset: opts.RequirePasswordReset,
	})
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return user, nil
}

// TODO: Support multiple sort field and filtering.
func (m *manager) List(ctx context.Context, opts entity.UserListOptions) ([]*entity.User, error) {
	// validated here to cover every caller; InvalidArgument becomes a 400
	if err := validate.Struct(opts); err != nil {
		return nil, errors.InvalidArgument.Wrap(err)
	}
	users, err := m.repository.ListUsers(ctx, opts)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return users, nil
}

func (m *manager) Delete(ctx context.Context, userIDs []int32) error {
	if sliceutil.In(preset.AdminUserID, userIDs...) {
		return errors.Forbidden.Newf("Cannot delete system built-in default admin user")
	}
	err := m.repository.Transaction(ctx, func(tctx context.Context) error {
		for _, userID := range userIDs {
			if err := m.repository.DeleteUser(tctx, userID); err != nil {
				return errors.Wrap(err)
			}
		}
		return nil
	})
	if err != nil {
		return errors.Wrap(err)
	}
	// the auth service ends the deleted users' sessions on this
	m.publish(ctx, message.LocalUsersDeleted{UserIDs: userIDs})
	return nil
}

func (m *manager) Get(ctx context.Context, userID int32) (*entity.User, error) {
	user, err := m.repository.GetUser(ctx, userID)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return user, nil
}

func (m *manager) Update(ctx context.Context, userID int32, opts entity.UserUpdateOptions) (*entity.User, error) {
	if opts.Username != nil && *opts.Username == "" {
		return nil, errors.InvalidArgument.Newf("Invalid Arguments: Username cannot be empty")
	}
	if userID == preset.AdminUserID && opts.Role != nil && *opts.Role != rbac.RoleAdmin {
		return nil, errors.Forbidden.Newf("Cannot update the built-in admin role to non-admin.")
	}
	var before, after *entity.User
	err := m.repository.Transaction(ctx, func(tctx context.Context) error {
		var err error
		if before, err = m.repository.GetUser(tctx, userID); err != nil {
			return errors.Wrap(err)
		}
		if after, err = m.repository.UpdateUser(tctx, userID, opts); err != nil {
			return errors.Wrap(err)
		}
		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err)
	}
	// credentials carry username and role, so auth ends sessions when either changes
	if before.Username != after.Username || before.Role != after.Role {
		m.publish(ctx, message.LocalUserIdentityChanged{UserID: userID})
	}
	return after, nil
}

func (m *manager) ResetPassword(ctx context.Context, userID int32, opts entity.UserResetPasswordOptions) error {
	if opts.Password == "" {
		return errors.InvalidArgument.Newf("Invalid Arguments: Password cannot be empty")
	}
	hash, err := hashPassword(opts.Password)
	if err != nil {
		return errors.Wrap(err)
	}
	if err := m.repository.SetUserPasswordHash(ctx, userID, hash); err != nil {
		return errors.Wrapf(err, "failed to reset user password")
	}
	// the auth service ends the user's sessions on this
	m.publish(ctx, message.LocalUserPasswordReset{UserID: userID})
	return nil
}

func (m *manager) ChangePassword(ctx context.Context, userID int32, opts entity.UserChangePasswordOptions) error {
	if opts.Password == "" {
		return errors.InvalidArgument.Newf("Invalid Arguments: Password cannot be empty")
	}
	if opts.OldPassword == "" {
		return errors.InvalidArgument.Newf("Invalid Arguments: Old password cannot be empty")
	}
	// hashed before the transaction so bcrypt's time is not spent holding it
	hash, err := hashPassword(opts.Password)
	if err != nil {
		return errors.Wrap(err)
	}
	err = m.repository.Transaction(ctx, func(tctx context.Context) error {
		current, err := m.repository.GetUserPasswordHash(tctx, userID)
		if err != nil {
			return errors.Wrap(err)
		}
		if !checkPassword(current, opts.OldPassword) {
			return errors.Forbidden.Newf("Access Denied: Old password validation failed.")
		}
		return m.repository.SetUserPasswordHash(tctx, userID, hash)
	})
	if err != nil {
		return errors.Wrapf(err, "failed to change user password")
	}
	// the auth service ends the user's sessions on this
	m.publish(ctx, message.LocalUserPasswordReset{UserID: userID})
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

func (m *manager) Validate(ctx context.Context, organization, username string) (bool, error) {
	organizationID, err := m.organizationID(ctx, organization)
	if err != nil {
		return false, errors.Unauthorized.Newf("Failed to validate username")
	}
	if _, err := m.repository.GetUserByName(ctx, organizationID, username); err != nil {
		return false, errors.Unauthorized.Newf("Failed to validate username")
	}
	return true, nil
}

func (m *manager) Authenticate(ctx context.Context, organization, username, password string) (*entity.Credential, error) {
	const authFailed = "Failed to auth user %s, username & password pair does not match any record"

	organizationID, err := m.organizationID(ctx, organization)
	if err != nil {
		// timing decoy, result discarded
		_ = checkPassword(dummyHash(), password)
		return nil, errors.Unauthorized.Newf(authFailed, username)
	}
	user, err := m.repository.GetUserByName(ctx, organizationID, username)
	if err != nil {
		// timing decoy, result discarded
		_ = checkPassword(dummyHash(), password)
		return nil, errors.Unauthorized.Newf(authFailed, username)
	}
	hash, err := m.repository.GetUserPasswordHash(ctx, user.ID)
	if err != nil {
		// timing decoy, result discarded
		_ = checkPassword(dummyHash(), password)
		return nil, errors.Unauthorized.Newf(authFailed, username)
	}
	if !checkPassword(hash, password) {
		return nil, errors.Unauthorized.Newf(authFailed, username)
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
