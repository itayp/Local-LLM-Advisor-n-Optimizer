package archtest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// repoRoot is the module root, from this package's directory.
const repoRoot = "../.."

// sourceFile is one parsed, non-test Go file of the daemon.
type sourceFile struct {
	rel  string // slash-separated, from the repo root: "internal/server/server.go"
	pkg  string // the directory: "internal/server"
	fset *token.FileSet
	file *ast.File
	// imports maps each imported path to the name it is used by in this
	// file ("net/http" → "http", or its alias).
	imports map[string]string
}

// daemonSources parses every non-test Go file under cmd/ and internal/ —
// what ships in the binary — except this package's own.
func daemonSources(t *testing.T) []sourceFile {
	t.Helper()
	var out []sourceFile
	for _, top := range []string{"cmd", "internal"} {
		root := filepath.Join(repoRoot, top)
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "testdata" || d.Name() == "node_modules" || d.Name() == "dist" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, err := filepath.Rel(repoRoot, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if strings.HasPrefix(rel, "internal/archtest/") {
				return nil
			}
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
			if err != nil {
				return err
			}
			sf := sourceFile{rel: rel, pkg: filepath.ToSlash(filepath.Dir(rel)), fset: fset, file: f, imports: map[string]string{}}
			for _, imp := range f.Imports {
				p, _ := strconv.Unquote(imp.Path.Value)
				name := p[strings.LastIndex(p, "/")+1:]
				if imp.Name != nil {
					name = imp.Name.Name
				}
				sf.imports[p] = name
			}
			out = append(out, sf)
			return nil
		})
		if err != nil {
			t.Fatalf("reading %s: %v", root, err)
		}
	}
	if len(out) < 50 {
		t.Fatalf("found only %d source files under cmd/ and internal/; is repoRoot right?", len(out))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].rel < out[j].rel })
	return out
}

// pos is "file:line" for a node.
func (sf sourceFile) pos(n ast.Node) string {
	p := sf.fset.Position(n.Pos())
	return sf.rel + ":" + strconv.Itoa(p.Line)
}

// selectorOf reports whether n is pkgName.Sel for the package imported as
// importPath in this file, and returns Sel.
func (sf sourceFile) selectorOf(n ast.Node, importPath string) (string, bool) {
	sel, ok := n.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return "", false
	}
	name, imported := sf.imports[importPath]
	if !imported || id.Name != name || id.Obj != nil {
		return "", false
	}
	return sel.Sel.Name, true
}

// modulePath is the module's import path prefix (go.mod: module advisor).
const modulePath = "advisor/"
