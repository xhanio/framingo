package certificate

import (
	"archive/tar"
	"crypto"
	"crypto/x509"
	"io"
	"net/http"
	"os"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/utils/certutil"

	"github.com/xhanio/framingo/example/pkg/types/api"
	"github.com/xhanio/framingo/example/pkg/types/entity"
)

func (r *router) Upload(c api.Context) error {
	var body api.CertUploadBody
	if err := c.Bind(&body); err != nil {
		return errors.BadRequest.Wrap(err)
	}
	if err := c.Validate(&body); err != nil {
		return errors.Wrap(err)
	}
	if body.Type != entity.CertTypePKCS12 && body.Type != entity.CertTypeCertKey && body.Type != entity.CertTypeRemoteCert {
		return errors.BadRequest.Newf("unsupported certificate type %s", body.Type)
	}
	if body.Password == "null" {
		body.Password = ""
	}
	form, err := c.MultipartForm()
	if err != nil {
		return errors.BadRequest.Wrap(err)
	}
	certFiles := form.File["cert"]
	if len(certFiles) != 1 {
		return errors.BadRequest.Newf("should have one and only one cert file")
	}
	certFile, err := certFiles[0].Open()
	if err != nil {
		return errors.BadRequest.Wrap(err)
	}
	defer certFile.Close()
	certContent, err := io.ReadAll(certFile)
	if err != nil {
		return errors.BadRequest.Wrap(err)
	}
	var keyContent []byte
	if body.Type == entity.CertTypeCertKey {
		keyFiles := form.File["key"]
		if len(keyFiles) != 1 {
			return errors.BadRequest.Newf("missing or multiple key files for Certificate type")
		}
		keyFile, err := keyFiles[0].Open()
		if err != nil {
			return errors.BadRequest.Wrap(err)
		}
		defer keyFile.Close()
		keyContent, err = io.ReadAll(keyFile)
		if err != nil {
			return errors.BadRequest.Wrap(err)
		}
	}
	var cert *x509.Certificate
	var pool []*x509.Certificate
	var key crypto.PrivateKey
	switch body.Type {
	case entity.CertTypePKCS12:
		cert, pool, key, err = certutil.ParsePKCS12(certContent, body.Password)
		if err != nil {
			return errors.InvalidArgument.Wrapf(err, "failed to parse pkcs12 file")
		}
	case entity.CertTypeRemoteCert:
		cert, pool, err = certutil.ParsePEMCert(certContent)
		if err != nil || cert == nil {
			cert, err = certutil.ParseDERCert(certContent)
		}
		if err != nil {
			return errors.InvalidArgument.Wrapf(err, "failed to parse cert file")
		}
	case entity.CertTypeCertKey:
		cert, pool, err = certutil.ParsePEMCert(certContent)
		if err != nil || cert == nil {
			cert, err = certutil.ParseDERCert(certContent)
		}
		if err != nil {
			return errors.InvalidArgument.Wrapf(err, "failed to parse cert file")
		}
		key, err = certutil.ParsePEMKey(keyContent, body.Password)
		if err != nil || key == nil {
			key, err = certutil.ParseDERKey(keyContent, body.Password)
		}
		if err != nil {
			return errors.InvalidArgument.Wrapf(err, "failed to parse key file")
		}
	}
	bundle, err := certutil.NewCert(cert, pool, key)
	if err != nil {
		return errors.InvalidArgument.Wrap(err)
	}
	if body.IsCA && !bundle.Cert().IsCA {
		return errors.InvalidArgument.Newf("certificate has to be a CA")
	}
	if body.Type == entity.CertTypeRemoteCert && !bundle.Cert().IsCA {
		return errors.InvalidArgument.Newf("unsupported certificate type %s for non-CA", body.Type)
	}
	if body.Type == entity.CertTypeCertKey && bundle.Key() == nil {
		return errors.InvalidArgument.Newf("local certificate must have a valid key")
	}
	opts := entity.CertCreateOptions{
		Name:    body.Name,
		IsCA:    body.IsCA,
		IsLocal: bundle.Key() != nil,
		Type:    body.Type,
		Bundle:  bundle,
	}
	result, err := r.cm.Import(c, opts)
	if err != nil {
		return errors.Wrap(err)
	}
	return c.JSON(http.StatusOK, api.CertUploadGenerateResponse{CertID: result.ID})
}

func (r *router) List(c api.Context) error {
	opts := entity.CertListOptions{}
	switch c.QueryParam("is_ca") {
	case "true":
		v := true
		opts.IsCA = &v
	case "false":
		v := false
		opts.IsCA = &v
	}
	if c.QueryParam("is_local") == "true" {
		opts.IsLocal = true
	}
	certs, err := r.cm.List(c, opts)
	if err != nil {
		return errors.Wrap(err)
	}
	if certs == nil {
		certs = []*entity.Certificate{}
	}
	return c.JSON(http.StatusOK, certs)
}

func (r *router) Delete(c api.Context) error {
	certIDs := []int32{}
	if err := c.BindQuery().MustInt32s("ids", &certIDs).BindError(); err != nil {
		return errors.BadRequest.Wrap(err)
	}
	if err := r.cm.Delete(c, certIDs); err != nil {
		return errors.Wrap(err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (r *router) Get(c api.Context) error {
	var certID int32
	if err := c.BindPath().MustInt32("id", &certID).BindError(); err != nil {
		return errors.BadRequest.Wrap(err)
	}
	cert, err := r.cm.Get(c, certID)
	if err != nil {
		return errors.Wrap(err)
	}
	return c.JSON(http.StatusOK, cert)
}

func (r *router) Update(c api.Context) error {
	var certID int32
	if err := c.BindPath().MustInt32("id", &certID).BindError(); err != nil {
		return errors.BadRequest.Wrap(err)
	}
	var body api.CertUpdateBody
	if err := c.Bind(&body); err != nil {
		return errors.BadRequest.Wrap(err)
	}
	opts := entity.CertUpdateOptions{Comments: body.Comments}
	if err := r.cm.Update(c, certID, opts); err != nil {
		return errors.Wrap(err)
	}
	return c.NoContent(http.StatusOK)
}

func (r *router) Download(c api.Context) error {
	var certID int32
	if err := c.BindPath().MustInt32("id", &certID).BindError(); err != nil {
		return errors.BadRequest.Wrap(err)
	}
	cert, err := r.cm.Get(c, certID)
	if err != nil {
		return errors.Wrap(err)
	}
	var tempFile *os.File
	defer func() {
		if tempFile != nil {
			tempFile.Close()
			if err := os.Remove(tempFile.Name()); err != nil {
				r.log.Warnf("failed to remove temp file %s", tempFile.Name())
			}
		}
	}()
	switch cert.UploadType {
	case entity.CertTypePKCS12:
		tempFile, err = os.CreateTemp("", "certificate-*.p12")
		if err != nil {
			return errors.Wrap(err)
		}
		if _, err := tempFile.Write(cert.Bundle.CertPEM()); err != nil {
			return errors.Wrap(err)
		}
		c.Response().Header().Set("Content-Disposition", "attachment; filename="+cert.Name+".p12")
		c.Response().Header().Set("Content-Type", "application/x-pkcs12")
	case entity.CertTypeCertKey:
		tempFile, err = os.CreateTemp("", "cert-key-bundle-*.tar")
		if err != nil {
			return errors.Wrap(err)
		}
		tw := tar.NewWriter(tempFile)
		defer tw.Close()
		for _, data := range [][]byte{cert.Bundle.CertPEM(), cert.Bundle.KeyPEM()} {
			if err := tw.WriteHeader(&tar.Header{Name: cert.Name, Mode: 0600, Size: int64(len(data))}); err != nil {
				return errors.Wrap(err)
			}
			if _, err := tw.Write(data); err != nil {
				return errors.Wrap(err)
			}
		}
		c.Response().Header().Set("Content-Disposition", "attachment; filename="+cert.Name+".tar")
	default:
		tempFile, err = os.CreateTemp("", "certificate-*.pem")
		if err != nil {
			return errors.Wrap(err)
		}
		if _, err := tempFile.Write(cert.Bundle.CertPEM()); err != nil {
			return errors.Wrap(err)
		}
		c.Response().Header().Set("Content-Disposition", "attachment; filename="+cert.Name+".pem")
	}
	return c.File(tempFile.Name())
}

func (r *router) Generate(c api.Context) error {
	var body api.CertGenerateBody
	if err := c.Bind(&body); err != nil {
		return errors.BadRequest.Wrap(err)
	}
	if err := c.Validate(&body); err != nil {
		return errors.Wrap(err)
	}
	opts := entity.CertIssueOptions{
		Name:           body.Name,
		CommonName:     body.CommonName,
		SubjectAltName: body.SubjectAltName,
		CAID:           body.CAID,
	}
	cert, err := r.cm.Issue(c, opts)
	if err != nil {
		return errors.Wrap(err)
	}
	return c.JSON(http.StatusOK, api.CertUploadGenerateResponse{CertID: cert.ID})
}

func (r *router) Handlers() map[string]any {
	return api.DiscoverHandlers(r)
}
