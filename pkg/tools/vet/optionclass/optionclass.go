// Package optionclass reports an option in an option.go that says nothing of its
// class: construction, fixed at New, or reloadable, the default of a key Init
// re-reads at every start (design/config.md).
package optionclass

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/xhanio/framingo/pkg/tools/vet/directive"
)

const (
	construction = "// Construction options:"
	reloadable   = "// Reloadable options:"
)

var Analyzer = &analysis.Analyzer{
	Name: "optionclass",
	Doc:  "reports an option in an option.go under no Construction options or Reloadable options comment",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		name := filepath.Base(pass.Fset.Position(file.Pos()).Filename)
		if name != "option.go" && name != "options.go" {
			continue
		}
		headers := classHeaders(file)
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Recv != nil || !fd.Name.IsExported() || !strings.HasPrefix(fd.Name.Name, "With") {
				continue
			}
			if !under(headers, fd.Pos()) {
				directive.Reportf(pass, fd.Name.Pos(), "option %s is in no class: put it under a %q or %q comment (design/config.md)",
					fd.Name.Name, strings.TrimPrefix(construction, "// "), strings.TrimPrefix(reloadable, "// "))
			}
		}
	}
	return nil, nil
}

// classHeaders are where the file's class comments stand.
func classHeaders(file *ast.File) []token.Pos {
	var at []token.Pos
	for _, group := range file.Comments {
		for _, c := range group.List {
			if strings.HasPrefix(c.Text, construction) || strings.HasPrefix(c.Text, reloadable) {
				at = append(at, c.Pos())
			}
		}
	}
	return at
}

// under: a class comment stands before pos, the nearest above an option being its class.
func under(headers []token.Pos, pos token.Pos) bool {
	for _, h := range headers {
		if h < pos {
			return true
		}
	}
	return false
}
