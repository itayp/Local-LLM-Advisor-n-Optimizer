package archtest

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"
	"testing"
)

// TestOllamaModelsIsNamedOnlyWhereItIsRead holds the reading half of D-72 in
// the code: the advisor sets no environment variable, and the models folder
// is what Ollama reports. The name OLLAMA_MODELS appears in a string only in
// internal/hardware (the fallback reading, "before Ollama has run") and
// internal/backend/ollama (reading Ollama's own log line). A package that
// names it anywhere else — the server building a check from it, a settings
// writer — fails here, and has to argue for it in ARCHITECTURE.md first.
func TestOllamaModelsIsNamedOnlyWhereItIsRead(t *testing.T) {
	allowed := map[string]bool{"internal/hardware": true, "internal/backend/ollama": true}
	for _, sf := range daemonSources(t) {
		if allowed[sf.pkg] {
			continue
		}
		ast.Inspect(sf.file, func(n ast.Node) bool {
			if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if s, err := strconv.Unquote(lit.Value); err == nil && strings.Contains(s, "OLLAMA_MODELS") {
					t.Errorf("%s: names OLLAMA_MODELS; only internal/hardware and internal/backend/ollama read it (D-72)", sf.pos(lit))
				}
			}
			return true
		})
	}
}
