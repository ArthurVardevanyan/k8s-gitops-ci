// Package upstreamref extracts and digests functions from Kubernetes
// upstream source, so that a runtime check's claim to be a 1:1 port of a
// specific upstream validation rule can be verified mechanically.
//
// Two failure modes are detected:
//
//   - The cited function no longer exists (renamed, removed, or relocated).
//     Upstream does this regularly: validateResourceRequirements did not
//     exist under that name in v1.30.
//   - The cited function still exists but its body changed since the ref was
//     validated. Existence alone is not enough: ValidateServiceCreate kept
//     its name between v1.30 and v1.37 while its behavior changed
//     substantially (the KEP-5311 Service name relaxation).
//
// Digests are taken over a normalized rendering of the function - comments
// stripped and formatting canonicalized - so that documentation churn and
// gofmt changes do not cause spurious failures while real logic changes do.
package upstreamref

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"sort"
	"strings"
)

// Digest returns "sha256:<hex>" over the normalized source of the named
// functions in src, which must be the contents of a Go file.
//
// Functions are emitted in sorted name order rather than source order, so a
// ref's digest does not change when upstream merely moves a function around
// within its file.
func Digest(src []byte, functions []string) (string, error) {
	rendered, err := Render(src, functions)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(rendered))
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// Render returns the normalized source of the named functions, concatenated
// in sorted name order. It is exported so that a digest mismatch can be
// explained with a readable diff rather than just two hashes.
func Render(src []byte, functions []string) (string, error) {
	fset := token.NewFileSet()
	// Comments are deliberately not parsed: ParseFile without ParseComments
	// drops them from the AST, which is exactly the normalization we want.
	file, err := parser.ParseFile(fset, "upstream.go", src, 0)
	if err != nil {
		return "", fmt.Errorf("parse upstream source: %w", err)
	}

	found := map[string]string{}
	for _, decl := range file.Decls {
		name, ok := declName(decl)
		if !ok {
			continue
		}
		if !contains(functions, name) {
			continue
		}
		var b strings.Builder
		// Printed against a fresh, empty FileSet rather than the one the
		// source was parsed with. The printer uses position information to
		// decide where to preserve blank lines, so printing with the parse
		// FileSet leaks the source's line structure into the output: a
		// dropped comment leaves the blank line it occupied, and adding or
		// removing a blank line moves the digest.
		//
		// That defeated the normalization this digest exists to provide.
		// Upstream edits doc comments on every release, so citations would
		// fail for reasons unrelated to validation behaviour - and a check
		// that cries wolf every release is one whose failures get waved
		// through, including the real ones.
		//
		// An empty FileSet resolves every position as unknown, so the
		// printer emits its canonical layout and the digest depends only on
		// the syntax tree.
		if err := (&printer.Config{Mode: printer.RawFormat, Tabwidth: 8}).Fprint(&b, token.NewFileSet(), decl); err != nil {
			return "", fmt.Errorf("render %s: %w", name, err)
		}
		found[name] = b.String()
	}

	var missing []string
	for _, fn := range functions {
		if _, ok := found[fn]; !ok {
			missing = append(missing, fn)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return "", &MissingError{Functions: missing}
	}

	names := make([]string, 0, len(found))
	for n := range found {
		names = append(names, n)
	}
	sort.Strings(names)

	var out strings.Builder
	for _, n := range names {
		out.WriteString(found[n])
		out.WriteString("\n")
	}
	return out.String(), nil
}

// MissingError reports cited functions that are absent from the upstream
// source. This is the strongest possible signal that a check is not the 1:1
// port it claims to be - either the citation was wrong to begin with, or
// upstream removed the rule and the check now enforces something the API
// server does not.
type MissingError struct {
	Functions []string
}

func (e *MissingError) Error() string {
	return "function(s) not found in upstream source: " + strings.Join(e.Functions, ", ")
}

// declName returns the name a top-level declaration binds, for the forms a
// validation rule can take upstream: a function declaration, or a package
// level var alias such as `var ValidateServiceName = NameIsDNS1035Label`.
func declName(decl ast.Decl) (string, bool) {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		if d.Recv != nil {
			return "", false
		}
		return d.Name.Name, true
	case *ast.GenDecl:
		if d.Tok != token.VAR {
			return "", false
		}
		for _, spec := range d.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok || len(vs.Names) == 0 {
				continue
			}
			return vs.Names[0].Name, true
		}
	}
	return "", false
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}
