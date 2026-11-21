// Package framingo holds the framingo command's subcommands.
package framingo

import (
	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "framingo",
		Short: "Check a project built on the framingo framework",
	}
	cmd.AddCommand(NewVerifyCmd())
	cmd.AddCommand(NewVersionCmd())
	return cmd
}
