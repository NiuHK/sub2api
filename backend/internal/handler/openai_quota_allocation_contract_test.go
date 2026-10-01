package handler

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// Keep the quota gate at every synchronous OpenAI account-selection entrypoint.
// These endpoints each have an independent failover loop, so a service-level
// check on Responses/Messages alone would leave a bypass through the other
// protocol handlers.
func TestOpenAIQuotaAllocationGateCoversAllOpenAISelectionEntrypoints(t *testing.T) {
	cases := map[string]string{
		"openai_chat_completions.go": "chatCompletionsSingle",
		"openai_images.go":           "Images",
		"openai_embeddings.go":       "Embeddings",
		"openai_alpha_search.go":     "AlphaSearch",
	}
	for filename, functionName := range cases {
		t.Run(filename, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, filepath.Join(".", filename), nil, 0)
			require.NoError(t, err)

			var target *ast.FuncDecl
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if ok && fn.Name.Name == functionName {
					target = fn
					break
				}
			}
			require.NotNil(t, target, "selection entrypoint must exist")

			gateCalls := 0
			releaseCalls := 0
			ast.Inspect(target.Body, func(node ast.Node) bool {
				switch expr := node.(type) {
				case *ast.CallExpr:
					if selector, ok := expr.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "checkOpenAIQuotaAllocation" {
						gateCalls++
					}
				case *ast.SelectorExpr:
					if expr.Sel.Name == "ReleaseFunc" {
						releaseCalls++
					}
				}
				return true
			})
			require.Greater(t, gateCalls, 0, "entrypoint must check quota after account selection")
			require.Greater(t, releaseCalls, 0, "entrypoint must release selection on quota rejection")
		})
	}
}
