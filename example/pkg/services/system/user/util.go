package user

import (
	"context"

	"github.com/xhanio/framingo/example/pkg/types/preset"
	"github.com/xhanio/framingo/pkg/types/common"
)

// the default organization is a preset, not a row, so it never hits the database
func (m *manager) organizationID(ctx context.Context, organization string) (int32, error) {
	if organization == preset.DefaultOrganizationName {
		return preset.DefaultOrganizationID, nil
	}
	return m.repository.GetOrganizationID(ctx, organization)
}

// call only after commit; dropped with no sender
func (m *manager) publish(ctx context.Context, msg common.Message) {
	if m.sender != nil {
		m.sender.SendMessage(ctx, m, msg)
	}
}
