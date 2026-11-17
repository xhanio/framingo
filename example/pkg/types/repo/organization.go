package repo

import (
	"context"

	"github.com/xhanio/framingo/example/pkg/types/entity"
)

type Organization interface {
	Repository
	CreateOrganization(ctx context.Context, opts entity.OrganizationCreateOptions) (*entity.Organization, error)
	GetOrganization(ctx context.Context, organizationID int32) (*entity.Organization, error)
	GetOrganizationID(ctx context.Context, organizationName string) (int32, error)
	UpdateOrganization(ctx context.Context, organizationID int32, opts entity.OrganizationUpdateOptions) (*entity.Organization, error)
	DeleteOrganization(ctx context.Context, organizationID int32) error
}
