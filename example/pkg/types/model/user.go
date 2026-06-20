package model

import (
	"context"

	"github.com/xhanio/framingo/pkg/types/common"

	"github.com/xhanio/framingo/example/pkg/types/entity"
)

type User interface {
	common.Service
	Create(ctx context.Context, opts entity.UserCreateOptions) (*entity.User, error)
	List(ctx context.Context, opts entity.UserListOptions) ([]*entity.User, error)
	Delete(ctx context.Context, userIDs []int32) error
	Get(ctx context.Context, userID int32) (*entity.User, error)
	Update(ctx context.Context, userID int32, opts entity.UserUpdateOptions) (*entity.User, error)
	ResetPassword(ctx context.Context, isResetOwnPwd bool, userID int32, opts entity.UserResetPasswordOptions) error
	// GetSAMLMetadata(ctx context.Context) error
	// UpdateSAMLSettings(ctx context.Context) error
	// DeleteSAMLSettings(ctx context.Context) error
}
