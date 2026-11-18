package app

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/example/pkg/components/server/example"
)

// NewMigrateCmd is its own step so a deployment migrates once, not on every replica start.
func NewMigrateCmd() *cobra.Command {
	var migrateConfig string
	cmd := &cobra.Command{
		Use:          "migrate",
		Short:        "Apply the database migrations db.migration names, then exit",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.Wrap(example.Migrate(context.Background(), migrateConfig))
		},
	}
	cmd.Flags().StringVarP(&migrateConfig, "config", "c", "config.yaml", "config file path")
	return cmd
}
