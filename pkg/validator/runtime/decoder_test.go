package runtime

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestRuntimeChecksDoNotDecodeWithEncodingJSON guards the decoder contract
// for the whole check family.
//
// Checks are handed raw manifest bytes, which in this pipeline are YAML.
// encoding/json cannot parse YAML, so a check decoding with json.Unmarshal
// fails on every real document and - because these checks treat a decode
// failure as "not my kind" and return nil - silently reports nothing.
//
// That failure is invisible in every way that normally catches a bug: the
// check is registered, runs, passes, and is non-exemptable so no one
// reviews suppressions for it. Only the absence of findings would reveal
// it, and absence is what a passing run looks like. Hence a structural
// rule: use sigs.k8s.io/yaml, which accepts JSON too.
func TestRuntimeChecksDoNotDecodeWithEncodingJSON(t *testing.T) {
	fset := token.NewFileSet()
	var offenders []string

	// Only *decoding* is the problem. json.Marshal is legitimate here -
	// walker.go re-encodes an already-decoded map before handing it to the
	// YAML decoder - so match the decode entry points specifically rather
	// than the import, which would flag that valid use.
	decodeFuncs := map[string]bool{
		"Unmarshal":     true,
		"NewDecoder":    true,
		"UnmarshalJSON": true,
	}

	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return perr
		}
		// Resolve the local name bound to "encoding/json" in this file.
		jsonName := ""
		for _, imp := range f.Imports {
			p, uerr := strconv.Unquote(imp.Path.Value)
			if uerr != nil || p != "encoding/json" {
				continue
			}
			jsonName = "json"
			if imp.Name != nil {
				jsonName = imp.Name.Name
			}
		}
		if jsonName == "" {
			return nil
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := sel.X.(*ast.Ident)
			if !ok || ident.Name != jsonName || !decodeFuncs[sel.Sel.Name] {
				return true
			}
			offenders = append(offenders,
				fmt.Sprintf("%s:%d (%s.%s)", path, fset.Position(call.Pos()).Line, jsonName, sel.Sel.Name))
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walking runtime sources: %v", err)
	}

	for _, o := range offenders {
		t.Errorf("%s decodes with encoding/json; runtime checks receive YAML, which encoding/json "+
			"cannot parse, so every check in this file would silently return no findings. "+
			"Use sigs.k8s.io/yaml instead.", o)
	}
}
