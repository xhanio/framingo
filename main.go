// Command framingo checks a project built on the framework.
//
//	go install github.com/xhanio/framingo@latest
package main

import (
	"os"

	"github.com/xhanio/framingo/pkg/components/cmd/framingo"
	"github.com/xhanio/framingo/pkg/tools/vet"
)

func main() {
	// go vet runs this binary as its -vettool, once per package
	if vet.AsTool(os.Args[1:]) {
		vet.Main()
	}
	// cobra reports a command's error itself; verify has already said what failed
	if err := framingo.NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
