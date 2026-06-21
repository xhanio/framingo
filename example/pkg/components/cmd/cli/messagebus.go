package cli

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/xhanio/errors"
)

func NewMessageStreamCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "stream",
		Short:        "Watch and print messages from the message bus",
		RunE:         runMessageStream,
		SilenceUsage: true,
	}
	return cmd
}

func runMessageStream(cmd *cobra.Command, args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := cli.StreamMessages(ctx); err != nil {
		return errors.Wrap(err)
	}
	return nil
}
