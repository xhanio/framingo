package example

import (
	"context"
	"encoding/json"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/services/api/client"
	fapi "github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"

	"github.com/xhanio/framingo/example/pkg/types/api"
)

type Credential struct {
	SessionID string `json:"session_id"`
}

func (c *Credential) Dump(path string) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return errors.Wrap(err)
	}
	if err := os.WriteFile(path, b, 0644); err != nil {
		return errors.Wrap(err)
	}
	return nil
}

func (c *Credential) Load(path string) error {
	if path == "" {
		return nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return errors.Wrap(err)
	}
	if err := json.Unmarshal(b, c); err != nil {
		return errors.Wrap(err)
	}
	return nil
}

func (c *cli) Login(ctx context.Context, username, password string) error {
	body := &api.LoginRequest{
		Username: username,
		Password: password,
	}
	resp, err := c.cli.Send(ctx, &client.Request{
		Method:      http.MethodPost,
		Path:        "/auth/login",
		ContentType: echo.MIMEApplicationJSON,
		Body:        body,
	})
	if err != nil {
		return errors.Wrap(err)
	}
	defer resp.Body.Close()
	sessionID := resp.Header.Get(fapi.HeaderKeySession)
	if sessionID == "" {
		return errors.Unauthorized.Newf("empty session id")
	}
	c.cred.SessionID = sessionID
	if c.credFile != "" {
		if err := c.cred.Dump(c.credFile); err != nil {
			return errors.Wrap(err)
		}
	}
	c.cli.SetHeaders(common.NewPair(fapi.HeaderKeySession, c.cred.SessionID))
	return nil
}

func (c *cli) Logout(ctx context.Context) error {
	resp, err := c.cli.Send(ctx, &client.Request{
		Method:      http.MethodPost,
		Path:        "/auth/logout",
		ContentType: echo.MIMEApplicationJSON,
	})
	if err != nil {
		return errors.Wrap(err)
	}
	defer resp.Body.Close()
	if c.credFile != "" {
		if err := os.Remove(c.credFile); err != nil && !os.IsNotExist(err) {
			return errors.Wrap(err)
		}
	}
	return nil
}
