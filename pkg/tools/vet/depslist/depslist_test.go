package depslist_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/xhanio/framingo/pkg/tools/vet/depslist"
)

func TestAnalyzer(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), depslist.Analyzer, "a")
}
