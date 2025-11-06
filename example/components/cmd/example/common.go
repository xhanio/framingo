package example

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/xhanio/framingo/pkg/types/info"
)

func NewVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "version",
		Run: runVersion,
	}
	return cmd
}

func runVersion(cmd *cobra.Command, args []string) {
	buildInfo := info.GetBuildInfo()
	b, _ := json.MarshalIndent(&buildInfo, "", "  ")
	fmt.Println(string(b))
}
