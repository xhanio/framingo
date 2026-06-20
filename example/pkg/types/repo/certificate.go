package repo

import (
	"context"

	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/orm"
)

type Certificate interface {
	CreateCertificate(ctx context.Context, opts entity.CertCreateOptions) (*orm.Certificate, error)
	GetCertificate(ctx context.Context, certID int32) (*orm.Certificate, error)
	GetCertificateByName(ctx context.Context, certName string) (*orm.Certificate, error)
	ListCertificates(ctx context.Context, opts entity.CertListOptions) ([]*orm.Certificate, error)
	UpdateCertificateComments(ctx context.Context, certID int32, comments string) error
	IncCertificateRefCount(ctx context.Context, certID int32) error
	DecCertificateRefCount(ctx context.Context, certID int32) error
	DeleteCertificate(ctx context.Context, certID int32) error
}
