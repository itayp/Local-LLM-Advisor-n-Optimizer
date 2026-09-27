package archtest

import (
	"go/ast"
	"reflect"
	"strings"
	"testing"

	"advisor/internal/backend"
	"advisor/internal/suite"
)

// benchPkg is the benchmark harness: the only code that asks a model for
// anything (D-8: the advisor calls no LLM to do its own job; the adapter's
// Generate is used by the benchmark harness only).
const benchPkg = "internal/bench"

// TestOnlyTheBenchmarkAsksAModelAnything holds D-8 in the code: a call to
// a runtime's Generate, or a GenerateRequest being built, anywhere but the
// benchmark harness fails the build.
func TestOnlyTheBenchmarkAsksAModelAnything(t *testing.T) {
	for _, sf := range daemonSources(t) {
		if sf.pkg == benchPkg {
			continue
		}
		inBackend := sf.pkg == "internal/backend" || strings.HasPrefix(sf.pkg, "internal/backend/")
		ast.Inspect(sf.file, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CallExpr:
				if sel, ok := x.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Generate" && !inBackend {
					t.Errorf("%s: calls Generate; only the benchmark harness (%s) asks a model anything (D-8)", sf.pos(x), benchPkg)
				}
			case *ast.CompositeLit:
				if name, ok := sf.selectorOf(x.Type, "advisor/internal/backend"); ok && name == "GenerateRequest" {
					t.Errorf("%s: builds a backend.GenerateRequest; only the benchmark harness does (D-8)", sf.pos(x))
				}
			}
			return true
		})
	}
}

// TestAModelIsSentOnlyTheSuitesText holds D-65 in the types: what a
// runtime's Generate receives has no field that can carry text from
// anywhere but the embedded benchmark suite. A new field — a system prompt,
// a free-text option, a map — fails here and has to be argued for in
// ARCHITECTURE.md first.
func TestAModelIsSentOnlyTheSuitesText(t *testing.T) {
	want := map[string]reflect.Type{
		"Model":      reflect.TypeOf(""),              // the runtime's name for an installed model
		"Prompt":     reflect.TypeOf(suite.Prompt{}),  // only the embedded suite makes one
		"Options":    reflect.TypeOf(suite.Options{}), // numbers
		"KeepAlive":  reflect.TypeOf(""),              // the harness's own configuration
		"Raw":        reflect.TypeOf(false),
		"NoTruncate": reflect.TypeOf(false),
	}
	rt := reflect.TypeOf(backend.GenerateRequest{})
	if rt.NumField() != len(want) {
		t.Errorf("backend.GenerateRequest has %d fields, want exactly %d", rt.NumField(), len(want))
	}
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		w, ok := want[f.Name]
		if !ok {
			t.Errorf("backend.GenerateRequest.%s (%s) is new: a field that reaches a model must be argued for in ARCHITECTURE.md (D-65) and added here", f.Name, f.Type)
			continue
		}
		if f.Type != w {
			t.Errorf("backend.GenerateRequest.%s is %s, want %s", f.Name, f.Type, w)
		}
	}
	// A Prompt's text is not reachable from outside its package.
	pt := reflect.TypeOf(suite.Prompt{})
	for i := 0; i < pt.NumField(); i++ {
		if pt.Field(i).IsExported() {
			t.Errorf("suite.Prompt.%s is exported: anyone could put text in a prompt", pt.Field(i).Name)
		}
	}
	// The options are numbers.
	ot := reflect.TypeOf(suite.Options{})
	for i := 0; i < ot.NumField(); i++ {
		switch ot.Field(i).Type.Kind() {
		case reflect.Int, reflect.Int64, reflect.Float64, reflect.Float32, reflect.Int32:
		default:
			t.Errorf("suite.Options.%s is a %s; options sent to a model are numbers only", ot.Field(i).Name, ot.Field(i).Type)
		}
	}
}
