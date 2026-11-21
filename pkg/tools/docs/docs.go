// Package docs checks markdown against what it points at and what it copies:
// a relative link reaches a file, and its anchor a heading; a Go snippet
// marked with its source says what the source says.
package docs

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/xhanio/errors"
)

var (
	link    = regexp.MustCompile(`\]\(([^)\s]+)\)`)
	heading = regexp.MustCompile(`^#+\s+(.*)$`)
	inline  = regexp.MustCompile("`[^`]*`")
	// <!-- source: path/from/the/root.go --> on the line above a ```go fence
	source = regexp.MustCompile(`^<!--\s*source:\s*(\S+)\s*-->$`)
)

// a markdown file's lines, with what is inside a code fence told apart
type document struct {
	path  string
	lines []string
	code  []bool // lines[i] is a fence or inside one
}

// Links reports, as "file:line: message", each relative link under root whose
// file is missing or whose anchor is no heading of it. skip names directories
// left out, by their path from root: their documents are not read, and a link
// into one is reported, since what is left out - ignored by git, generated -
// is there for no other reader.
func Links(root string, skip ...string) ([]string, error) {
	docs, err := read(root, skip)
	if err != nil {
		return nil, err
	}
	anchors := map[string]map[string]bool{}
	for _, d := range docs {
		anchors[d.path] = d.anchors()
	}
	var problems []string
	for _, d := range docs {
		for i, line := range d.lines {
			if d.code[i] {
				continue
			}
			for _, m := range link.FindAllStringSubmatch(inline.ReplaceAllString(line, ""), -1) {
				target := m[1]
				if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
					continue
				}
				file, anchor, _ := strings.Cut(target, "#")
				path := d.path
				if file != "" {
					path = filepath.Join(filepath.Dir(d.path), file)
				}
				if dir := within(path, skip); dir != "" {
					problems = append(problems, fmt.Sprintf("%s:%d: link to %s: %s is left out of the docs", d.path, i+1, target, dir))
					continue
				}
				if _, err := os.Stat(filepath.Join(root, path)); err != nil {
					problems = append(problems, fmt.Sprintf("%s:%d: link to %s: no such file", d.path, i+1, target))
					continue
				}
				if known, isDoc := anchors[path]; anchor != "" && isDoc && !known[anchor] {
					problems = append(problems, fmt.Sprintf("%s:%d: link to %s: no heading #%s in %s", d.path, i+1, target, anchor, path))
				}
			}
		}
	}
	return problems, nil
}

// Snippets reports each Go snippet under root, marked with
// "<!-- source: path -->" on the line above its fence, whose functions or
// types differ from those of the same name in that file - comments and blank
// lines aside. A snippet may show part of the file, never another version of it.
func Snippets(root string, skip ...string) ([]string, error) {
	docs, err := read(root, skip)
	if err != nil {
		return nil, err
	}
	var problems []string
	for _, d := range docs {
		for i := 0; i+1 < len(d.lines); i++ {
			m := source.FindStringSubmatch(strings.TrimSpace(d.lines[i]))
			if m == nil {
				continue
			}
			at := fmt.Sprintf("%s:%d", d.path, i+1)
			if !strings.HasPrefix(d.lines[i+1], "```go") {
				problems = append(problems, at+": a source marker with no ```go fence on the next line")
				continue
			}
			var snippet []string
			for j := i + 2; j < len(d.lines) && !strings.HasPrefix(d.lines[j], "```"); j++ {
				snippet = append(snippet, d.lines[j])
			}
			for _, p := range compare(strings.Join(snippet, "\n"), filepath.Join(root, m[1])) {
				problems = append(problems, fmt.Sprintf("%s: %s %s", at, p, m[1]))
			}
		}
	}
	return problems, nil
}

// within is the skipped directory path lies in, if any.
func within(path string, skip []string) string {
	for _, s := range skip {
		s = filepath.Clean(s)
		if path == s || strings.HasPrefix(path, s+string(filepath.Separator)) {
			return s
		}
	}
	return ""
}

func read(root string, skip []string) ([]*document, error) {
	skipped := map[string]bool{}
	for _, s := range skip {
		skipped[filepath.Clean(s)] = true
	}
	var docs []*document
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if skipped[rel] || (rel != "." && strings.HasPrefix(entry.Name(), ".")) || entry.Name() == "testdata" || entry.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".md" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		d := &document{path: rel, lines: strings.Split(string(data), "\n")}
		d.code = make([]bool, len(d.lines))
		fenced := false
		for i, line := range d.lines {
			fence := strings.HasPrefix(strings.TrimSpace(line), "```")
			d.code[i] = fenced || fence
			if fence {
				fenced = !fenced
			}
		}
		docs = append(docs, d)
		return nil
	})
	return docs, errors.Wrap(err)
}

// anchors are the document's headings as GitHub links to them: lower case,
// spaces to hyphens, punctuation dropped.
func (d *document) anchors() map[string]bool {
	found := map[string]bool{}
	for i, line := range d.lines {
		if d.code[i] {
			continue
		}
		if m := heading.FindStringSubmatch(line); m != nil {
			var b strings.Builder
			for _, r := range strings.ToLower(strings.TrimSpace(m[1])) {
				switch {
				case unicode.IsLetter(r), unicode.IsDigit(r), r == '-', r == '_':
					b.WriteRune(r)
				case r == ' ':
					b.WriteRune('-')
				}
			}
			found[b.String()] = true
		}
	}
	return found
}

// compare reports each function or type the snippet declares that the file
// does not declare the same.
func compare(snippet, file string) []string {
	if !strings.HasPrefix(strings.TrimSpace(snippet), "package ") {
		snippet = "package snippet\n" + snippet
	}
	shown, err := declarations("snippet.go", snippet)
	if err != nil {
		return []string{fmt.Sprintf("the snippet does not parse (%v), so it cannot be held to", err)}
	}
	actual, err := declarations(file, nil)
	if err != nil {
		return []string{fmt.Sprintf("cannot read the source (%v):", err)}
	}
	var problems []string
	for _, name := range shown.order {
		code, ok := actual.code[name]
		switch {
		case !ok:
			problems = append(problems, name+" is not in")
		case code != shown.code[name]:
			problems = append(problems, name+" differs from")
		}
	}
	return problems
}

type decls struct {
	order []string
	code  map[string]string // by name, printed without comments or blank lines
}

func declarations(filename string, src any) (*decls, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	found := &decls{code: map[string]string{}}
	add := func(name string, node any) {
		var buf bytes.Buffer
		if err := printer.Fprint(&buf, fset, node); err != nil {
			return
		}
		var lines []string
		for _, line := range strings.Split(buf.String(), "\n") {
			if strings.TrimSpace(line) != "" {
				lines = append(lines, line)
			}
		}
		found.order = append(found.order, name)
		found.code[name] = strings.Join(lines, "\n")
	}
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			name := "func " + d.Name.Name
			if d.Recv != nil && len(d.Recv.List) == 1 {
				var recv bytes.Buffer
				if err := printer.Fprint(&recv, fset, d.Recv.List[0].Type); err == nil {
					name = "func (" + recv.String() + ") " + d.Name.Name
				}
			}
			add(name, d)
		case *ast.GenDecl:
			if d.Tok != token.TYPE {
				continue
			}
			for _, spec := range d.Specs {
				ts := spec.(*ast.TypeSpec)
				add("type "+ts.Name.Name, ts)
			}
		}
	}
	return found, nil
}
