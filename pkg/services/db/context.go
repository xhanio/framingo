package db

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/xhanio/framingo/pkg/types/common"
)

func (m *manager) FromContext(ctx context.Context) *gorm.DB {
	tx, ok := ctx.Value(common.ContextKeyTX).(*gorm.DB)
	if !ok {
		return m.ormDB.WithContext(ctx)
	}
	return tx
}

func (m *manager) FromContextTimeout(ctx context.Context, timeout time.Duration) (*gorm.DB, context.CancelFunc) {
	if timeout == 0 {
		timeout = m.connection.ExecTimeout
	}
	tx := m.FromContext(ctx)
	_ctx, cancel := context.WithTimeout(ctx, timeout)
	return tx.WithContext(_ctx), cancel
}

func WrapContext(ctx context.Context, tx *gorm.DB) context.Context {
	return context.WithValue(ctx, common.ContextKeyTX, tx)
}
