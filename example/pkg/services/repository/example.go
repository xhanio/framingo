package repository

import (
	"context"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/example/pkg/types/orm"
)

func (m *manager) CreateHelloWorld(ctx context.Context, message string) (*orm.HelloWorld, error) {
	tx := m.db.FromContext(ctx)
	ormModel := &orm.HelloWorld{Message: message}
	if err := tx.Create(ormModel).Error; err != nil {
		return nil, errors.DBFailed.Wrap(err)
	}
	return ormModel, nil
}
