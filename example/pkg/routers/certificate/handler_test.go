package certificate

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func download(t *testing.T, cert *entity.Certificate) *httptest.ResponseRecorder {
	t.Helper()
	r := newRouter(oneCert{cert: cert}, log.Default)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(httptest.NewRequest(http.MethodPost, "/certificates/1/download", nil), rec)
	c.SetParamNames("id")
	c.SetParamValues("1")
	require.NoError(t, api.WrapHandler(r.Download)(c))
	require.Equal(t, http.StatusOK, rec.Code)
	return rec
}

func serverCert(t *testing.T) certutil.CertBundle {
	t.Helper()
	ca, err := certutil.New()
	require.NoError(t, err)
	b, err := ca.SignServer(&certutil.ServerRequest{CommonName: "srv.local"})
	require.NoError(t, err)
	return b
}

// A certificate with its key downloads as a whole tar of the two, each its own file.
func TestDownloadCertAndKey(t *testing.T) {
	b := serverCert(t)
	rec := download(t, &entity.Certificate{Name: "srv", UploadType: entity.CertTypeCertKey, Bundle: b})
	assert.Equal(t, "attachment; filename=srv.tar", rec.Header().Get("Content-Disposition"))
	assert.Equal(t, "application/x-tar", rec.Header().Get("Content-Type"))

	body := rec.Body.Bytes()
	assert.True(t, bytes.HasSuffix(body, make([]byte, 1024)), "the archive ends with its two zero blocks")
	files := map[string]string{}
	tr := tar.NewReader(bytes.NewReader(body))
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		data, err := io.ReadAll(tr)
		require.NoError(t, err)
		files[h.Name] = string(data)
	}
	assert.Equal(t, map[string]string{"srv.crt": string(b.CertPEM()), "srv.key": string(b.KeyPEM())}, files)
}

func TestDownloadCertOnly(t *testing.T) {
	b := serverCert(t)
	rec := download(t, &entity.Certificate{Name: "agent-7", UploadType: entity.CertTypeRemoteCert, Bundle: b})
	assert.Equal(t, "attachment; filename=agent-7.pem", rec.Header().Get("Content-Disposition"))
	assert.Equal(t, string(b.CertPEM()), rec.Body.String())
}
