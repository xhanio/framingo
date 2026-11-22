package authnuser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xhanio/framingo/pkg/structs/lease"
	fapi "github.com/xhanio/framingo/pkg/types/api"

	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/model"
)

func TestNameDefaultsToPackage(t *testing.T) {
	assert.Equal(t, "authnuser", New(nil).Name())
}

func TestWithNameOverrides(t *testing.T) {
	assert.Equal(t, "authnuser2", New(nil, WithName("authnuser2")).Name())
	assert.Equal(t, "authnuser", New(nil, WithName("")).Name())
}

type fakeAuth struct {
	model.Auth
	token   *entity.Credential
	session *entity.Session
}

func (f fakeAuth) AuthenticateAPIToken(context.Context, string) (*entity.Credential, error) {
	return f.token, nil
}

func (f fakeAuth) GetSession(context.Context, string) (*entity.Session, bool) {
	return f.session, f.session != nil
}

// fakeLease takes the refresh the middleware gives a session.
type fakeLease struct{ lease.Lease }

func (fakeLease) Refresh(time.Duration) bool { return true }

// serve runs the middleware on a request with header set, returning the request's context
// the next handler got, and the echo.Context.
func serve(t *testing.T, auth model.Auth, header, value string) (context.Context, echo.Context) {
	t.Helper()
	mw, err := New(auth).Func(true, nil)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(header, value)
	c := echo.New().NewContext(req, httptest.NewRecorder())
	var got context.Context
	require.NoError(t, mw(func(c echo.Context) error {
		got = c.Request().Context()
		return nil
	})(c))
	return got, c
}

// What authentication found goes in the request's context, which any context the request
// hands on carries, after the handler returned too, not in echo's pooled store
// (design/request-context.md).
func TestAuthenticationPutsWhatItFoundInTheRequestsContext(t *testing.T) {
	token := &entity.Credential{UserName: "token"}
	got, c := serve(t, fakeAuth{token: token}, fapi.HeaderKeyAPIToken, "key")
	cred, ok := entity.CredentialFrom(got)
	assert.True(t, ok && cred == token, "the token's credential")
	assert.Nil(t, c.Get(fapi.ContextKeyCredential), "not in echo's store")

	session := &entity.Session{ID: "s", Credential: &entity.Credential{UserName: "session"}, Lease: fakeLease{}}
	got, c = serve(t, fakeAuth{session: session}, fapi.HeaderKeySession, "s")
	s, ok := entity.SessionFrom(got)
	assert.True(t, ok && s == session, "the session")
	cred, ok = entity.CredentialFrom(got)
	assert.True(t, ok && cred == session.Credential, "the session's credential")
	assert.Nil(t, c.Get(fapi.ContextKeySession), "not in echo's store")
}
