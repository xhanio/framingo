package cli

import (
	"os"
	"os/user"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/example/pkg/components/client/example"
)

var (
	help     bool
	verbose  bool
	endpoint string
	credFile string

	cli example.Client
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if help {
				_ = cmd.Help()
				os.Exit(0)
			}
			cu, err := user.Current()
			if err != nil {
				return errors.Wrap(err)
			}
			credFile = filepath.Join(cu.HomeDir, ".example")
			opts := []example.Option{
				example.WithCredential(credFile),
				example.WithEndpoint(endpoint),
			}
			if verbose {
				opts = append(opts, example.WithDebug())
			}
			cli = example.New(opts...)
			if err := cli.Init(); err != nil {
				return errors.Wrap(err)
			}
			return nil
		},
	}
	root.PersistentFlags().BoolVar(&help, "help", false, "")
	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "")
	root.PersistentFlags().StringVarP(&endpoint, "endpoint", "e", "", "server endpoint")

	root.AddCommand(NewVersionCmd())
	root.AddCommand(NewLoginCmd())
	root.AddCommand(NewLogoutCmd())
	root.AddCommand(NewHelloworldCmd())
	root.AddCommand(NewCertGenCmd())
	return root
}
