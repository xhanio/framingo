package repository

import (
	"context"
	"encoding/json"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/orm"
)

func (m *manager) CreateUser(ctx context.Context, opts entity.UserCreateOptions) (*orm.User, *orm.Contact, error) {
	tx := m.db.FromContext(ctx)
	var user *orm.User
	var contact *orm.Contact
	err := tx.Transaction(func(itx *gorm.DB) error {
		// Create user contact record
		contact = &orm.Contact{
			Email:          opts.Email,
			FirstName:      opts.FirstName,
			LastName:       opts.LastName,
			OrganizationID: opts.OrganizationID,
			Title:          opts.Title,
		}
		if err := itx.Create(&contact).Error; err != nil {
			switch err {
			case gorm.ErrDuplicatedKey:
				return errors.AlreadyExist.Wrapf(err, "user contact %s already exists", opts.Email)
			default:
				return errors.DBFailed.Wrap(err)
			}
		}
		// Create user record
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(opts.Password), bcrypt.DefaultCost)
		if err != nil {
			return errors.Wrap(err)
		}
		user = &orm.User{
			OrganizationID:       opts.OrganizationID,
			Username:             opts.Username,
			Password:             string(hashedPassword),
			Role:                 opts.Role,
			ContactID:            contact.ID,
			Expired:              opts.ChangePassword,
			RequirePasswordReset: opts.RequirePasswordReset,
		}
		if err := itx.Create(&user).Error; err != nil {
			switch err {
			case gorm.ErrDuplicatedKey:
				return errors.AlreadyExist.Wrapf(err, "user %s already exists", opts.Username)
			default:
				return errors.DBFailed.Wrap(err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to create user")
	}
	return user, contact, nil
}

func (m *manager) getUser(tx *gorm.DB, userID int32) (*orm.User, *orm.Contact, error) {
	var user *orm.User
	var contact *orm.Contact
	err := tx.Transaction(func(itx *gorm.DB) error {
		if err := itx.Model(&user).Where("id = ?", userID).First(&user).Error; err != nil {
			switch err {
			case gorm.ErrRecordNotFound:
				return errors.NotFound.Wrap(err)
			default:
				return errors.DBFailed.Wrap(err)
			}
		}
		c, err := m.getContact(itx, user.ContactID)
		if err != nil {
			if !errors.Is(err, errors.NotFound) {
				return errors.Wrap(err)
			}
		}
		contact = c
		return nil
	})
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to get user %d", userID)
	}
	return user, contact, nil
}

func (m *manager) GetUser(ctx context.Context, userID int32) (*orm.User, *orm.Contact, error) {
	tx := m.db.FromContext(ctx)
	return m.getUser(tx, userID)
}

func (m *manager) ListUsers(ctx context.Context, opts entity.UserListOptions) ([]*orm.User, []*orm.Contact, error) {
	tx := m.db.FromContext(ctx)
	var users []*orm.User
	var contacts []*orm.Contact
	err := tx.Transaction(func(itx *gorm.DB) error {
		query := itx.Model(&orm.User{})
		if opts.SortBy != "" {
			query = query.Order(clause.OrderByColumn{Column: clause.Column{Name: opts.SortBy}, Desc: opts.Desc})
		}
		// If organization ID is not default ViewAllOrganizationID (0), but some speficic organization ID
		if opts.OrganizationID != 0 {
			query = query.Where("organization_id = ?", opts.OrganizationID)
		}
		if err := query.Find(&users).Error; err != nil {
			switch err {
			default:
				return errors.DBFailed.Wrap(err)
			}
		}
		for _, user := range users {
			// Get contact information for each user
			contact, err := m.getContact(itx, user.ContactID)
			if err != nil {
				if !errors.Is(err, errors.NotFound) {
					return errors.Wrap(err)
				}
			}
			contacts = append(contacts, contact)
		}
		return nil
	})
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to list users")
	}
	return users, contacts, nil
}

func (m *manager) deleteUser(tx *gorm.DB, userID int32) error {
	return tx.Transaction(func(itx *gorm.DB) error {
		_, contact, err := m.getUser(itx, userID)
		if err != nil {
			return errors.Wrap(err)
		}
		if err := itx.Model(&orm.User{}).Where("id = ?", userID).Delete(&orm.User{}).Error; err != nil {
			return errors.DBFailed.Wrap(err)
		}
		if contact != nil {
			if _, err := m.deleteContact(itx, contact.ID); err != nil {
				if !errors.Is(err, errors.NotFound) {
					return errors.Wrap(err)
				}
			}
		}
		return nil
	})
}

func (m *manager) DeleteUser(ctx context.Context, userID int32) error {
	if err := m.deleteUser(m.db.FromContext(ctx), userID); err != nil {
		return errors.Wrapf(err, "failed to delete user %d", userID)
	}
	return nil
}

func (m *manager) DeleteUsers(ctx context.Context, userIDs []int32) error {
	tx := m.db.FromContext(ctx)
	err := tx.Transaction(func(itx *gorm.DB) error {
		for _, userID := range userIDs {
			if err := m.deleteUser(itx, userID); err != nil {
				return errors.Wrap(err)
			}
		}
		return nil
	})
	if err != nil {
		return errors.Wrapf(err, "failed to delete users")
	}
	return nil
}

func (m *manager) deleteContact(tx *gorm.DB, contactID int32) (*orm.Contact, error) {
	var contact *orm.Contact
	if err := tx.Model(&contact).Where("id = ?", contactID).Delete(&contact).Error; err != nil {
		switch err {
		case gorm.ErrRecordNotFound:
			return nil, errors.NotFound.Wrap(err)
		default:
			return nil, errors.DBFailed.Wrap(err)
		}
	}
	return contact, nil
}

func (m *manager) getContact(tx *gorm.DB, contactID int32) (*orm.Contact, error) {
	var contact *orm.Contact
	if err := tx.Model(contact).Where("id = ?", contactID).First(contact).Error; err != nil {
		switch err {
		case gorm.ErrRecordNotFound:
			return nil, errors.NotFound.Wrapf(err, "contact id %d not found", contactID)
		default:
			return nil, errors.DBFailed.Wrap(err)
		}
	}
	return contact, nil
}

func (m *manager) GetContact(ctx context.Context, contactID int32) (*orm.Contact, error) {
	tx := m.db.FromContext(ctx)
	return m.getContact(tx, contactID)
}

func (m *manager) updateUser(tx *gorm.DB, user *orm.User, updateMap map[string]any) error {
	if err := tx.Model(&user).Select("username", "role", "expired").Updates(updateMap).Error; err != nil {
		switch err {
		case gorm.ErrDuplicatedKey:
			return errors.AlreadyExist.Wrapf(err, "user %d already exists", user.ID)
		default:
			return errors.DBFailed.Wrap(err)
		}
	}
	return nil
}

func (m *manager) updateContact(tx *gorm.DB, contact *orm.Contact, updateMap map[string]any) error {
	if err := tx.Model(&contact).Select("email", "first_name", "last_name", "title").Updates(updateMap).Error; err != nil {
		return errors.DBFailed.Wrap(err)
	}
	return nil
}

func updateOptionsToMap(opts entity.UserUpdateOptions) (map[string]any, error) {
	bytes, err := json.Marshal(opts)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	result := map[string]any{}
	if err := json.Unmarshal(bytes, &result); err != nil {
		return nil, errors.Wrap(err)
	}
	return result, nil
}

func (m *manager) UpdateUser(ctx context.Context, userID int32, opts entity.UserUpdateOptions) (*orm.User, *orm.Contact, error) {
	tx := m.db.FromContext(ctx)
	var user *orm.User
	var contact *orm.Contact
	err := tx.Transaction(func(itx *gorm.DB) error {
		u, c, err := m.getUser(itx, userID)
		if err != nil {
			return errors.Wrap(err)
		}
		updateMap, err := updateOptionsToMap(opts)
		if err != nil {
			return errors.Wrap(err)
		}
		if err := m.updateUser(itx, u, updateMap); err != nil {
			return errors.Wrap(err)
		}
		if c != nil {
			if err := m.updateContact(itx, c, updateMap); err != nil {
				return errors.Wrap(err)
			}
		}
		u, c, err = m.getUser(itx, userID)
		if err != nil {
			return errors.Wrap(err)
		}
		user = u
		contact = c
		return nil
	})
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to update user %d", userID)
	}
	return user, contact, nil
}

func (m *manager) ResetUserPassword(ctx context.Context, userID int32, plainPassword string) error {
	tx := m.db.FromContext(ctx)
	err := tx.Transaction(func(itx *gorm.DB) error {
		user, _, err := m.getUser(itx, userID)
		if err != nil {
			return errors.Wrap(err)
		}
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)
		if err != nil {
			return errors.Internal.Wrap(err)
		}
		user.Password = string(hashedPassword)
		user.RequirePasswordReset = false
		if err := itx.Save(&user).Error; err != nil {
			return errors.DBFailed.Wrap(err)
		}
		return nil
	})
	if err != nil {
		return errors.Wrapf(err, "failed to reset password for user %d", userID)
	}
	return nil
}

func (m *manager) GetUserByName(ctx context.Context, organizationID int32, username string) (*orm.User, error) {
	var user *orm.User
	tx := m.db.FromContext(ctx)
	if err := tx.Model(&user).Where("username = ? AND organization_id = ?", username, organizationID).First(&user).Error; err != nil {
		switch err {
		case gorm.ErrRecordNotFound:
			return nil, errors.NotFound.Wrapf(err, "user %s in organization %d not found", username, organizationID)
		default:
			return nil, errors.DBFailed.Wrap(err)
		}
	}
	return user, nil
}
