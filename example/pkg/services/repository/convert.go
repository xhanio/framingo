package repository

import (
	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/utils/certutil"

	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/orm"
)

// Rows become entities as they leave the database: the repo contracts speak
// entity only, so nothing above this package handles an orm model.

// toUser joins a user row with its contact row; a missing contact leaves the
// contact fields empty.
func toUser(u *orm.User, c *orm.Contact) *entity.User {
	if u == nil {
		return nil
	}
	if c == nil {
		c = &orm.Contact{}
	}
	return &entity.User{
		ID:                   u.ID,
		Username:             u.Username,
		Title:                c.Title,
		FirstName:            c.FirstName,
		LastName:             c.LastName,
		Email:                c.Email,
		Role:                 u.Role,
		ChangePassword:       u.Expired,
		OrganizationID:       u.OrganizationID,
		RequirePasswordReset: u.RequirePasswordReset,
	}
}

func toRole(r *orm.Role) *entity.Role {
	if r == nil {
		return nil
	}
	return &entity.Role{
		ID:          r.ID,
		Name:        r.Name,
		Description: r.Description,
	}
}

func toOrganization(o *orm.Organization) *entity.Organization {
	if o == nil {
		return nil
	}
	return &entity.Organization{
		ID:   o.ID,
		Name: o.Name,
	}
}

func toHelloWorld(h *orm.HelloWorld) *entity.HelloWorld {
	if h == nil {
		return nil
	}
	return &entity.HelloWorld{
		ID:        h.ID,
		Message:   h.Message,
		CreatedAt: h.CreatedAt,
		UpdatedAt: h.UpdatedAt,
	}
}

// toCertificate copies a certificate row and decodes its stored bundle - the
// inverse of the encoding CreateCertificate writes. What is read out of the
// certificate in that bundle (subject, serial, validity, fingerprint,
// extensions) is the certificate service's to work out, not stored here.
func toCertificate(c *orm.Certificate) (*entity.Certificate, error) {
	if c == nil {
		return nil, nil
	}
	bundle, err := certutil.Decode(c.CertBundle)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to decode cert bundle for cert %s", c.Name)
	}
	return &entity.Certificate{
		ID:         c.ID,
		Name:       c.Name,
		Comments:   c.Comments,
		Source:     c.Source,
		UploadType: c.Type,
		IsCA:       c.IsCA,
		IsLocal:    c.IsLocal,
		RefCount:   c.RefCount,
		Bundle:     bundle,
	}, nil
}
