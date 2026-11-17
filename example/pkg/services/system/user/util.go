package user

import (
	"context"

	"github.com/xhanio/errors"
	"golang.org/x/crypto/bcrypt"

	"github.com/xhanio/framingo/example/pkg/types/preset"
)

// hashPassword is how the service stores a password: the repository keeps the
// hash it is given and never sees the password itself.
func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", errors.Wrap(err)
	}
	return string(hash), nil
}

// organizationID resolves an organization name to its ID. The default
// organization is a preset, not a row - nothing seeds it into the
// organizations table - so its ID is known without asking the database.
func (m *manager) organizationID(ctx context.Context, organization string) (int32, error) {
	if organization == preset.DefaultOrganizationName {
		return preset.DefaultOrganizationID, nil
	}
	return m.repository.GetOrganizationID(ctx, organization)
}
