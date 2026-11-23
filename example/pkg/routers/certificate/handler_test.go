package certificate

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/utils/certutil"
	"github.com/xhanio/framingo/pkg/utils/log"

	"github.com/xhanio/framingo/example/pkg/types/api"
	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/model"
)

// oneCert serves Get alone; any other method panics, being nil.
type oneCert struct {
	model.Certificate
	cert *entity.Certificate
}

func (o oneCert) Get(context.Context, int32) (*entity.Certificate, error) { return o.cert, nil }

// tagValidator stands in for the api server's: a failed tag is InvalidArgument.
type tagValidator struct{ v *validator.Validate }

func (tv tagValidator) Validate(i any) error {
	if err := tv.v.Struct(i); err != nil {
		return errors.InvalidArgument.Wrap(err)
	}
	return nil
}

func download(t *testing.T, cert *entity.Certificate, body string) (*httptest.ResponseRecorder, error) {
	t.Helper()
	r := newRouter(oneCert{cert: cert}, log.Default)
	e := echo.New()
	e.Validator = tagValidator{v: validator.New()}
	req := httptest.NewRequest(http.MethodPost, "/certificates/1/download", strings.NewReader(body))
	if body != "" {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")
	return rec, api.WrapHandler(r.Download)(c)
}

func serverCert(t *testing.T) (*entity.Certificate, *entity.Certificate) {
	t.Helper()
	ca, err := certutil.New()
	require.NoError(t, err)
	withKey, err := ca.SignServer(&certutil.ServerRequest{CommonName: "srv.local", KeepChain: true})
	require.NoError(t, err)
	certOnly, err := certutil.NewCertBundleWithoutKey(withKey.CertPEM())
	require.NoError(t, err)
	return &entity.Certificate{Name: "srv", UploadType: entity.CertTypeCertKey, Bundle: withKey},
		&entity.Certificate{Name: "agent-7", UploadType: entity.CertTypeRemoteCert, Bundle: certOnly}
}

// With its key and no format asked, a certificate downloads as a whole tar of
// the two, each its own file.
func TestDownloadTar(t *testing.T) {
	withKey, _ := serverCert(t)
	rec, err := download(t, withKey, "")
	require.NoError(t, err)
	assert.Equal(t, "attachment; filename=srv.tar", rec.Header().Get("Content-Disposition"))
	assert.Equal(t, "application/x-tar", rec.Header().Get("Content-Type"))

	body := rec.Body.Bytes()
	assert.True(t, bytes.HasSuffix(body, make([]byte, 1024)), "the archive ends with its two zero blocks")
	files := map[string]string{}
	tr := tar.NewReader(bytes.NewReader(body))
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		data, err := io.ReadAll(tr)
		require.NoError(t, err)
		files[h.Name] = string(data)
		assert.WithinDuration(t, time.Now(), h.ModTime, time.Minute, "%s is dated", h.Name)
	}
	assert.Equal(t, map[string]string{"srv.crt": string(withKey.Bundle.CertPEM()), "srv.key": string(withKey.Bundle.KeyPEM())}, files)
}

func TestDownloadFilenameIsQuotedWhenItMustBe(t *testing.T) {
	_, certOnly := serverCert(t)
	certOnly.Name = "agent 7; v2"
	rec, err := download(t, certOnly, "")
	require.NoError(t, err)
	assert.Equal(t, `attachment; filename="agent 7; v2.pem"`, rec.Header().Get("Content-Disposition"))
}

func TestDownloadPEM(t *testing.T) {
	withKey, certOnly := serverCert(t)
	rec, err := download(t, certOnly, "")
	require.NoError(t, err)
	assert.Equal(t, "attachment; filename=agent-7.pem", rec.Header().Get("Content-Disposition"))
	assert.Equal(t, string(certOnly.Bundle.CertPEM()), rec.Body.String(), "pem when there is no key")

	rec, err = download(t, withKey, `{"format":"pem"}`)
	require.NoError(t, err)
	assert.Equal(t, string(withKey.Bundle.CertPEM()), rec.Body.String(), "the certificate alone, asked for")
	assert.NotContains(t, rec.Body.String(), "PRIVATE KEY")
}

// A p12 is encoded on download with the caller's password, whatever the
// certificate was uploaded as.
func TestDownloadPKCS12(t *testing.T) {
	withKey, _ := serverCert(t)
	rec, err := download(t, withKey, `{"format":"p12","password":"s3cret"}`)
	require.NoError(t, err)
	assert.Equal(t, "attachment; filename=srv.p12", rec.Header().Get("Content-Disposition"))
	assert.Equal(t, "application/x-pkcs12", rec.Header().Get("Content-Type"))

	cert, chain, key, err := certutil.ParsePKCS12(rec.Body.Bytes(), "s3cret")
	require.NoError(t, err)
	assert.True(t, withKey.Bundle.Cert().Equal(cert))
	assert.True(t, key.(interface{ Equal(crypto.PrivateKey) bool }).Equal(withKey.Bundle.Key()))
	assert.Len(t, chain, 1, "the chain travels with it")
}

func TestDownloadRefusals(t *testing.T) {
	withKey, certOnly := serverCert(t)
	for name, c := range map[string]struct {
		cert     *entity.Certificate
		body     string
		category errors.Category
	}{
		"a p12 without a password": {withKey, `{"format":"p12"}`, errors.InvalidArgument},
		"an unknown format":        {withKey, `{"format":"der"}`, errors.InvalidArgument},
		"a tar without a key":      {certOnly, `{"format":"tar"}`, errors.BadRequest},
		"a p12 without a key":      {certOnly, `{"format":"p12","password":"s3cret"}`, errors.BadRequest},
	} {
		_, err := download(t, c.cert, c.body)
		assert.True(t, err != nil && errors.Is(err, c.category), "%s: got %v", name, err)
	}
}
