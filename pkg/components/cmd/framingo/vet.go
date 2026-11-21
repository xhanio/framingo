package framingo

import (
	"os"
	"os/signal"

	"github.com/spf13/cobra"
	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/tools/vet"
	"github.com/xhanio/framingo/pkg/utils/cmdutil"
)

func NewVetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vet [packages]",
		Short: "Check packages (default: ./...) against the framework's design decisions",
		Long: "vet runs the framework's analyzers through go vet, each message naming the\n" +
			"design note it enforces:\n" + analyzerList(),
		// go vet has printed the findings
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runVet,
	}
	return cmd
}

func analyzerList() string {
	list := ""
	for _, a := range vet.Analyzers() {
		list += "  " + a.Name + "  " + a.Doc + "\n"
	}
	return list
}

func runVet(cmd *cobra.Command, args []string) error {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
	defer stop()
	self, err := os.Executable()
	if err != nil {
		return errors.Wrapf(err, "failed to locate the running binary")
	}
	return cmdutil.New("go", vet.Args(self, args...),
		cmdutil.WithContext(ctx),
		cmdutil.WithStdout(cmd.OutOrStdout()),
		cmdutil.WithStderr(cmd.ErrOrStderr()),
	).Run()
}
