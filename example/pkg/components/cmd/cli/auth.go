package cli

import (
	"context"
	"fmt"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/xhanio/errors"
	"golang.org/x/term"
)

var (
	username string
)

func NewLoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "login",
		RunE: runLogin,
	}
	cmd.Flags().StringVarP(&username, "username", "u", "admin", "username")
	return cmd
}

func runLogin(cmd *cobra.Command, args []string) error {
	fmt.Print("Enter password: ")
	pwd, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return errors.Wrap(err)
	}
	if err := cli.Login(context.Background(), username, strings.TrimSpace(string(pwd))); err != nil {
		return errors.Wrap(err)
	}
	return nil
}

func NewLogoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "logout",
		RunE: runLogout,
	}
	return cmd
}

func runLogout(cmd *cobra.Command, args []string) error {
	if err := cli.Logout(context.Background()); err != nil {
		return errors.Wrap(err)
	}
	return nil
}
