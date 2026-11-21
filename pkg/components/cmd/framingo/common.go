package framingo

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

func NewVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the framingo version this command was built from",
		Run:   runVersion,
	}
	return cmd
}

// the module version go install recorded; "(devel)" from a local build
func runVersion(cmd *cobra.Command, args []string) {
	version := "unknown"
	if bi, ok := debug.ReadBuildInfo(); ok {
		version = bi.Main.Version
	}
	fmt.Fprintln(cmd.OutOrStdout(), version)
}
