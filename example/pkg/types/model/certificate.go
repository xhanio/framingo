package model

import (
	"context"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/certutil"

	"github.com/xhanio/framingo/example/pkg/types/entity"
)

type Certificate interface {
	common.Service
	Import(ctx context.Context, opts entity.CertCreateOptions) (*entity.Certificate, error)
	Issue(ctx context.Context, opts entity.CertIssueOptions) (*entity.Certificate, error)
	List(ctx context.Context, opts entity.CertListOptions) ([]*entity.Certificate, error)
	Get(ctx context.Context, certID int32) (*entity.Certificate, error)
	Delete(ctx context.Context, certIDs []int32) error
	Update(ctx context.Context, certID int32, opts entity.CertUpdateOptions) error
	IncRefCount(ctx context.Context, certID int32) error
	DecRefCount(ctx context.Context, certID int32) error
	DefaultCA() (certutil.CABundle, error)
}
