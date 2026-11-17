package repo

import (
	"context"

	"github.com/xhanio/framingo/example/pkg/types/entity"
)

type Certificate interface {
	Repository
	CreateCertificate(ctx context.Context, opts entity.CertCreateOptions) (*entity.Certificate, error)
	GetCertificate(ctx context.Context, certID int32) (*entity.Certificate, error)
	GetCertificateByName(ctx context.Context, certName string) (*entity.Certificate, error)
	ListCertificates(ctx context.Context, opts entity.CertListOptions) ([]*entity.Certificate, error)
	UpdateCertificateComments(ctx context.Context, certID int32, comments string) error
	IncCertificateRefCount(ctx context.Context, certID int32) error
	DecCertificateRefCount(ctx context.Context, certID int32) error
	DeleteCertificate(ctx context.Context, certID int32) error
}
