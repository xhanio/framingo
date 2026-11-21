package platformfirst_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/xhanio/framingo/pkg/tools/vet/platformfirst"
)

func TestAnalyzer(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), platformfirst.Analyzer,
		"myapp/pkg/services/order", "myapp/pkg/services/legacy", "myapp/pkg/services/platform",
		"myapp/pkg/utils/thing", "github.com/xhanio/framingo/pkg/services/db")
}
