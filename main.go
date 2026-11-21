// Command framingo checks a project built on the framework.
//
//	go install github.com/xhanio/framingo@latest
package main

import (
	"os"

	"github.com/xhanio/framingo/pkg/components/cmd/framingo"
)

func main() {
	// cobra reports a command's error itself; verify has already said what failed
	if err := framingo.NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
