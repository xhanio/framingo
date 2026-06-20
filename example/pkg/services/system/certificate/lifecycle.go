package certificate

import (
	"context"

	"github.com/xhanio/errors"
)

func (m *manager) Init(ctx context.Context) error {
	_, err := m.DefaultCA()
	return errors.Wrapf(err, "Failed to reload the default CA")
}
