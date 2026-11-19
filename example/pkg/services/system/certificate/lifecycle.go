package certificate

import (
	"context"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/example/pkg/types/preset"
)

// Init loads the system CAs, creating them on the first start; a restart reloads them.
func (m *manager) Init(ctx context.Context) error {
	server, err := m.loadSystemCA(ctx, preset.ServerCAName, "server CA")
	if err != nil {
		return errors.Wrapf(err, "Failed to load the server CA")
	}
	agent, err := m.loadSystemCA(ctx, preset.AgentCAName, "agent CA")
	if err != nil {
		return errors.Wrapf(err, "Failed to load the agent CA")
	}
	m.mu.Lock()
	m.serverCA, m.agentCA = server, agent
	m.mu.Unlock()
	return nil
}
