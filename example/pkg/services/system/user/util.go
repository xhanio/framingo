package user

import (
	"context"

	"github.com/xhanio/framingo/example/pkg/types/preset"
)

// organizationID resolves an organization name to its ID. The default
// organization is a preset, not a row - nothing seeds it into the
// organizations table - so its ID is known without asking the database.
func (m *manager) organizationID(ctx context.Context, organization string) (int32, error) {
	if organization == preset.DefaultOrganizationName {
		return preset.DefaultOrganizationID, nil
	}
	return m.repository.GetOrganizationID(ctx, organization)
}
