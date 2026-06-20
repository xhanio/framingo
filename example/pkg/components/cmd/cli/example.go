package cli

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/xhanio/errors"
)

func NewHelloworldCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "helloworld",
		RunE:         runHelloworld,
		SilenceUsage: true,
	}
	return cmd
}

func runHelloworld(cmd *cobra.Command, args []string) error {
	var message string
	if len(args) > 0 {
		message = args[0]
	}
	if err := cli.HelloWorld(context.Background(), message); err != nil {
		return errors.Wrap(err)
	}
	return nil
}
