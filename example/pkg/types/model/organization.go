package model

import (
	"context"

	"github.com/xhanio/framingo/pkg/types/common"

	"github.com/xhanio/framingo/example/pkg/types/entity"
)

type Organization interface {
	common.Service
	Create(ctx context.Context, opts entity.OrganizationCreateOptions) error
	Get(ctx context.Context, organizationID int32) (*entity.Organization, error)
	Delete(ctx context.Context, organizationID int32) error
	Update(ctx context.Context, organizationID int32, opts entity.OrganizationUpdateOptions) error
}
