package example

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/example/pkg/components/server/example"
)

var (
	configPath string
)

func NewDaemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "daemon",
		RunE:         runDaemon,
		SilenceUsage: true,
	}
	cmd.PersistentFlags().StringVarP(&configPath, "config", "c", "config.json", "filter targets by regex")
	return cmd
}

func runDaemon(cmd *cobra.Command, args []string) error {
	m := example.New(example.Config{
		Path: configPath,
	})
	if err := m.Init(); err != nil {
		return errors.Wrap(err)
	}
	if err := m.Start(context.Background()); err != nil {
		return errors.Wrap(err)
	}
	return nil
}
