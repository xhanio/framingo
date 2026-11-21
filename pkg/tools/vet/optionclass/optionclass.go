// Package optionclass reports an option in an option.go with no class comment above
// it: construction, fixed at New, or reloadable, the default of a key every Init
// re-reads (design/config.md).
package optionclass

import (
	"go/ast"
	"go/token"
	"go/types"
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
	Doc:  "reports an option in an option.go — an exported func returning an …Option — under no Construction options or Reloadable options comment",
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
			if !ok || fd.Recv != nil || !fd.Name.IsExported() || !isOption(pass, fd) {
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

// isOption: fd returns one value of a type named …Option — Option, ServerOption — so
// FullDisplay and NoStdout count as WithName does.
func isOption(pass *analysis.Pass, fd *ast.FuncDecl) bool {
	results := fd.Type.Results
	if results == nil || results.NumFields() != 1 {
		return false
	}
	named, ok := pass.TypesInfo.TypeOf(results.List[0].Type).(*types.Named)
	return ok && strings.HasSuffix(named.Obj().Name(), "Option")
}

// classHeaders are the positions of the file's class comments.
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

// under reports whether a class comment stands before pos.
func under(headers []token.Pos, pos token.Pos) bool {
	for _, h := range headers {
		if h < pos {
			return true
		}
	}
	return false
}
