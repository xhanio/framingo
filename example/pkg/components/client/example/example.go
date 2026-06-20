package example

import (
	"context"
	"fmt"
	"io"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/services/api/client"
	fapi "github.com/xhanio/framingo/pkg/types/api"

	"github.com/xhanio/framingo/example/pkg/types/api"
)

func (c *cli) HelloWorld(ctx context.Context, message string) error {
	path := "/example/helloworld"
	body := &api.CreateHelloWorldMessage{Message: message}
	resp, err := c.cli.PostJSON(ctx, path, body, client.WithRequestEncoding(fapi.EncodingDeflate))
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
