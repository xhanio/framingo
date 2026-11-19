package certificate

import (
	"context"

	"github.com/xhanio/errors"
)

func (m *manager) Init(ctx context.Context) error {
	if _, err := m.ServerCA(); err != nil {
		return errors.Wrapf(err, "Failed to load the server CA")
	}
	if _, err := m.AgentCA(); err != nil {
		return errors.Wrapf(err, "Failed to load the agent CA")
	}
	return nil
}
