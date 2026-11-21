package optionclass_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/xhanio/framingo/pkg/tools/vet/optionclass"
)

func TestAnalyzer(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), optionclass.Analyzer, "a", "b")
}
