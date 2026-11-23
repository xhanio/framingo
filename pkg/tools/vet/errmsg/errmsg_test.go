package errmsg_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/xhanio/framingo/pkg/tools/vet/errmsg"
)

func TestAnalyzer(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), errmsg.Analyzer, "a")
}
