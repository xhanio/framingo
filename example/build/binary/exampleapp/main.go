package main

import (
	"fmt"
	"os"

	"github.com/xhanio/framingo/example/pkg/components/cmd/app"
)

func main() {
	rootCmd := app.NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
