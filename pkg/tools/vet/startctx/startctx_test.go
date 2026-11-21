package startctx_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/xhanio/framingo/pkg/tools/vet/startctx"
)

func TestAnalyzer(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), startctx.Analyzer, "a")
}
