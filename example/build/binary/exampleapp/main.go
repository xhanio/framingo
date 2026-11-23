package main

import (
	"fmt"
	"os"

	"github.com/xhanio/framingo/example/pkg/components/cmd/app"
)

func main() {
	rootCmd := app.NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %+v\n", err) // the daemon's last word: keep the stack
		os.Exit(1)
	}
}
