package db

import (
	"context"
	"database/sql"
	"fmt"
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

func (m *manager) Transaction(ctx context.Context, fn func(ctx context.Context) error, opts ...*sql.TxOptions) error {
	tx := m.ormDB.WithContext(ctx).Begin(opts...)
	if tx.Error != nil {
		return tx.Error
	}

	txCtx := WrapContext(ctx, tx)

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	if err := fn(txCtx); err != nil {
		if rbErr := tx.Rollback().Error; rbErr != nil {
			return fmt.Errorf("tx rollback failed: %v, original error: %w", rbErr, err)
		}
		return err
	}

	return tx.Commit().Error
}
