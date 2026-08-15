package validator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
)

// writeFile writes content to dir/rel (creating parent dirs) and returns the
// full path.
func writeFile(t *testing.T, dir, rel, content string) string {
	t.Helper()
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return full
}

// placeholderFindingsOf returns the findings whose CheckID is "placeholder".
func placeholderFindingsOf(findings []check.Finding) []check.Finding {
	var out []check.Finding
	for _, f := range findings {
		if f.CheckID == "placeholder" {
			out = append(out, f)
		}
	}
	return out
}

// TestPlaceholderIsRenderSensitive proves the render-sensitive family opts
// into rendered-output evaluation while a raw-only check (large-file/etc.
// aren't registered ScopeDoc checks, so we assert on the registered set).
func TestRenderSensitiveClassification(t *testing.T) {
	wantSensitive := map[string]bool{
		"placeholder":      true,
		"image-checksum":   true,
		"image-fqdn":       true,
		"namespace":        true,
		"psa-labels":       true,
		"rbac-readonly":    true,
		"rbac-wildcards":   true,
		"crb":              true,
		"sync-options":     true,
		"named-ports":      true,
		"podspec-defaults": true,
	}
	for _, c := range check.ByScope(check.ScopeDoc) {
		got := check.IsRenderSensitive(c)
		if want := wantSensitive[c.ID()]; got != want {
			t.Errorf("check %q: RenderSensitive=%v, want %v", c.ID(), got, want)
		}
	}
}

// TestPlaceholderRawSuppressedByCoverage proves that a sentinel placeholder
// (e.g. `image: <PATCHED_BY_KUSTOMIZE>`) living in a file that IS covered by
// a rendered overlay is NOT reported by the raw pass - the rendered pass is
// authoritative for it - the placeholder false-positive fix.
func TestPlaceholderRawSuppressedByCoverage(t *testing.T) {
	d := t.TempDir()
	f := writeFile(t, d, "app/components/x/cm.yaml", "kind: ConfigMap\nmetadata:\n  name: x\ndata:\n  image: <PATCHED_BY_KUSTOMIZE>\n")

	// Raw pass with the file marked as covered by a rendered overlay:
	// render-sensitive checks (placeholder) must be skipped for it.
	covered := map[string]bool{filepath.Clean(f): true}
	res := runDocChecks([]string{f}, covered, nil, 1, nil)
	if got := placeholderFindingsOf(res.Findings); len(got) != 0 {
		t.Errorf("expected no raw placeholder findings for covered file, got %+v", got)
	}
}

// TestPlaceholderRawFallbackWhenUncovered proves a placeholder in a file NOT
// covered by any rendered overlay (a brand-new component not yet wired up)
// still gets flagged by the raw fallback tier - nothing is silently skipped.
func TestPlaceholderRawFallbackWhenUncovered(t *testing.T) {
	d := t.TempDir()
	f := writeFile(t, d, "app/components/x/cm.yaml", "kind: ConfigMap\nmetadata:\n  name: x\ndata:\n  token: <SOME_UNRESOLVED_VALUE>\n")

	// No coverage: render-sensitive checks fall back to running on the file.
	res := runDocChecks([]string{f}, nil, nil, 1, nil)
	if got := placeholderFindingsOf(res.Findings); len(got) == 0 {
		t.Errorf("expected raw placeholder finding for uncovered file, got none")
	}
}

// TestRenderedPassFlagsUnresolvedAVP proves the rendered pass enables AVP
// scanning (CheckAVP:true): a `<path:...>` reference surviving in rendered
// output is a genuine unresolved-secret failure and IS flagged, whereas the
// raw pass (CheckAVP:false) would not flag it.
func TestRenderedPassFlagsUnresolvedAVP(t *testing.T) {
	rendered := []renderedOverlay{{
		overlay: "app/overlays/cluster1",
		data:    []byte("kind: Secret\nmetadata:\n  name: s\nstringData:\n  password: <path:secret/data/foo#bar>\n"),
	}}
	res := runDocChecksRendered(rendered, nil, 1, nil)
	got := placeholderFindingsOf(res.Findings)
	if len(got) == 0 {
		t.Fatalf("expected rendered pass to flag surviving AVP reference, got none")
	}
	if got[0].File != "app/overlays/cluster1" {
		t.Errorf("expected finding attributed to overlay path, got %q", got[0].File)
	}
}

// TestRawPassIgnoresAVP proves the raw pass keeps CheckAVP:false: an AVP
// reference in raw source is the intended committed state, not a finding.
func TestRawPassIgnoresAVP(t *testing.T) {
	d := t.TempDir()
	f := writeFile(t, d, "app/base/secret.yaml", "kind: Secret\nmetadata:\n  name: s\nstringData:\n  password: <path:secret/data/foo#bar>\n")
	res := runDocChecks([]string{f}, nil, nil, 1, nil)
	if got := placeholderFindingsOf(res.Findings); len(got) != 0 {
		t.Errorf("expected raw pass to ignore AVP reference, got %+v", got)
	}
}

// TestRenderedFindingsNeverForcedDirect proves the rendered pass no longer
// blanket-marks findings ForcedDirect based on the overlay - resource-level
// blocking/warning classification is decided later by
// classifyResourceCompliance. This is the mechanism behind the resource-level
// attribution fix (a touched overlay kustomization.yaml no longer makes every
// base-derived finding blocking).
func TestRenderedFindingsNeverForcedDirect(t *testing.T) {
	rendered := []renderedOverlay{{
		overlay: "app/overlays/cluster1",
		data:    []byte("kind: Secret\nmetadata:\n  name: s\nstringData:\n  password: <path:secret/data/foo#bar>\n"),
	}}
	res := runDocChecksRendered(rendered, nil, 1, nil)
	got := placeholderFindingsOf(res.Findings)
	if len(got) == 0 {
		t.Fatal("expected a rendered finding")
	}
	if got[0].ForcedDirect {
		t.Errorf("expected ForcedDirect=false: rendered pass must not force direct classification")
	}
}

// TestClassifyResourceCompliance_ResourceLevelSplit proves resource-level
// attribution: a finding on a resource whose definition was NOT changed (only
// the overlay's kustomization.yaml) is non-blocking; a finding on a resource
// whose source WAS changed (and feeds the overlay) is blocking.
func TestClassifyResourceCompliance_ResourceLevelSplit(t *testing.T) {
	finding := check.Finding{CheckID: "podspec-defaults", Kind: "Job", Name: "j", File: "app/overlays/pd1010", Message: "schedulerName"}

	// Resource NOT changed (empty changedKeys) -> non-blocking warning.
	warnCtx := &complianceAttributionCtx{changedKeys: map[string][]string{}}
	blocking, nonblocking := classifyResourceCompliance([]check.Finding{finding}, warnCtx)
	if len(blocking["podspec-defaults"]) != 0 || len(nonblocking["podspec-defaults"]) != 1 {
		t.Errorf("expected the finding to be non-blocking when its resource was not changed, got blocking=%d warning=%d",
			len(blocking["podspec-defaults"]), len(nonblocking["podspec-defaults"]))
	}

	// Resource changed in a file that feeds this overlay -> blocking.
	blockCtx := &complianceAttributionCtx{
		changedKeys:    map[string][]string{"Job/j": {"app/overlays/pd1010/job.yaml"}},
		directOverlays: map[string]bool{"app/pd1010": true},
	}
	blocking, nonblocking = classifyResourceCompliance([]check.Finding{finding}, blockCtx)
	if len(blocking["podspec-defaults"]) != 1 || len(nonblocking["podspec-defaults"]) != 0 {
		t.Errorf("expected the finding to be blocking when its resource was directly changed, got blocking=%d warning=%d",
			len(blocking["podspec-defaults"]), len(nonblocking["podspec-defaults"]))
	}
}
