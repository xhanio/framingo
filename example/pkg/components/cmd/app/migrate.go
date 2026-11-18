package app

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/example/pkg/components/server/example"
)

// NewMigrateCmd applies the database migrations and exits - a step of its
// own, so a deployment runs it once (the prod Job) instead of every replica
// migrating on start.
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
