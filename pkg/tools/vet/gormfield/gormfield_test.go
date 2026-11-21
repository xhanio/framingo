package gormfield_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/xhanio/framingo/pkg/tools/vet/gormfield"
)

func TestAnalyzer(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), gormfield.Analyzer, "a", "myapp/pkg/services/db")
}
