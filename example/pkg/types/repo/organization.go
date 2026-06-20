package repo

import (
	"context"

	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/orm"
)

type Organization interface {
	CreateOrganization(ctx context.Context, opts entity.OrganizationCreateOptions) (*orm.Organization, error)
	GetOrganization(ctx context.Context, organizationID int32) (*orm.Organization, error)
	GetOrganizationID(ctx context.Context, organizationName string) (int32, error)
	UpdateOrganization(ctx context.Context, organizationID int32, opts entity.OrganizationUpdateOptions) (*orm.Organization, error)
	DeleteOrganization(ctx context.Context, organizationID int32) error
}
