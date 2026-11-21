package framingo

import (
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"github.com/xhanio/framingo/pkg/tools/verify"
)

func NewVerifyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify [dir...]",
		Short: "Format, build, vet, lint and test each module directory (default: .)",
		Long: "verify gives one verdict on a change: gofmt, go build, go vet, golangci-lint\n" +
			"and go test -race, for each module directory given. Every step runs even\n" +
			"after one fails, and it ends \"verified\" or lists what failed.",
		// the steps' own output says what failed
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runVerify,
	}
	return cmd
}

func runVerify(cmd *cobra.Command, args []string) error {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
	defer stop()
	return verify.Run(ctx, cmd.OutOrStdout(), args...)
}
