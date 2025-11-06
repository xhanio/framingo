package example

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	help    bool
	verbose bool
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		// SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if help {
				_ = cmd.Help()
				os.Exit(0)
			}
		},
	}
	root.PersistentFlags().BoolVar(&help, "help", false, "")
	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "")

	root.AddCommand(NewDaemonCmd())
	root.AddCommand(NewVersionCmd())
	return root
}
