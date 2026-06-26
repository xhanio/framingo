package example

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/services/api/client"
	fapi "github.com/xhanio/framingo/pkg/types/api"

	"github.com/xhanio/framingo/example/pkg/types/api"
)

func (c *cli) HelloWorld(ctx context.Context, message string) error {
	body := &api.HelloWorldCreateRequest{Message: message}
	resp, err := c.cli.Send(ctx, &client.Request{
		Method:      http.MethodPost,
		Path:        "/example/helloworld",
		ContentType: echo.MIMEApplicationJSON,
		Body:        body,
	}, client.WithRequestEncoding(fapi.EncodingDeflate))
	if err != nil {
		return errors.Wrap(err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err)
	}
	fmt.Println(string(b))
	return nil
}
