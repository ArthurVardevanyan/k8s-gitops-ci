package validator

import (
	"os"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
)

// TestAppRootOf covers the multi-segment app-root derivation: the app root is
// the prefix before "/overlays/" (or before "/base/"|"/components/"), never a
// hardcoded first path segment.
func TestAppRootOf(t *testing.T) {
	t.Parallel()
	cases := []struct {
		file        string
		wantApp     string
		wantCluster string
	}{
		// Single-segment app (legacy layout every prior test used).
		{"app/overlays/prod/x.yaml", "app", "prod"},
		{"app/base/x.yaml", "app", ""},
		// Multi-segment app (HomeLab: kubernetes/<app>/...).
		{"kubernetes/intel-gpu-monitor/overlays/okd/x.yaml", "kubernetes/intel-gpu-monitor", "okd"},
		{"kubernetes/intel-gpu-monitor/base/x.yaml", "kubernetes/intel-gpu-monitor", ""},
		{"kubernetes/intel-device-plugins/components/c/x.yaml", "kubernetes/intel-device-plugins", ""},
	}
	for _, c := range cases {
		gotApp, gotCluster := appRootOf(c.file)
		if gotApp != c.wantApp || gotCluster != c.wantCluster {
			t.Errorf("appRootOf(%q) = (%q,%q), want (%q,%q)",
				c.file, gotApp, gotCluster, c.wantApp, c.wantCluster)
		}
	}
}

// TestDirectlyChangedOverlays_MultiSegmentApp proves the direct-overlay key is
// built from the full app root, so a change under
// kubernetes/<app>/overlays/<cluster>/ is recorded as "<app-root>/<cluster>"
// and lines up with the key isResourceAffected constructs. Regression for the
// bug where parts[0] ("kubernetes") was hardcoded as the app.
func TestDirectlyChangedOverlays_MultiSegmentApp(t *testing.T) {
	t.Parallel()
	got := directlyChangedOverlays([]string{
		"kubernetes/intel-gpu-monitor/overlays/okd/deployment.yaml",
		"kubernetes/intel-gpu-monitor/base/kustomization.yaml", // base, not an overlay change
	})
	if !got["kubernetes/intel-gpu-monitor/okd"] {
		t.Errorf("expected direct-overlay key kubernetes/intel-gpu-monitor/okd, got %v", got)
	}
	if got["kubernetes/okd"] {
		t.Errorf("did not expect the hardcoded-parts[0] key kubernetes/okd, got %v", got)
	}
}

// TestClassifyResourceCompliance_MultiSegmentAppIsBlocking is the end-to-end
// regression for HomeLab PR #611: a brand-new resource added directly under a
// multi-segment overlay dir must classify as BLOCKING, not a pre-existing
// warning.
func TestClassifyResourceCompliance_MultiSegmentAppIsBlocking(t *testing.T) {
	t.Parallel()
	finding := check.Finding{
		CheckID: "image-checksum",
		Kind:    "Deployment",
		Name:    "intel-gpu-monitor",
		Value:   "registry.example.com/intel-gpu-monitor:not_latest",
		File:    "kubernetes/intel-gpu-monitor/overlays/okd",
	}
	src := "kubernetes/intel-gpu-monitor/overlays/okd/okd.yaml"
	ctx := &complianceAttributionCtx{
		changedKeys:    map[string][]string{"Deployment/intel-gpu-monitor": {src}},
		directOverlays: directlyChangedOverlays([]string{src}),
	}

	blocking, nonblocking := classifyResourceCompliance([]check.Finding{finding}, ctx)
	if len(blocking["image-checksum"]) != 1 || len(nonblocking["image-checksum"]) != 0 {
		t.Errorf("expected the new multi-segment-app resource to be blocking, got blocking=%d warning=%d",
			len(blocking["image-checksum"]), len(nonblocking["image-checksum"]))
	}
}

// TestClassifyResourceCompliance_MultiSegmentBaseUnchangedResourceIsWarning
// proves the inverse still holds: a finding whose resource was NOT changed (its
// Kind/Name isn't in changedKeys) stays a non-blocking pre-existing warning,
// even for a multi-segment app.
func TestClassifyResourceCompliance_MultiSegmentBaseUnchangedResourceIsWarning(t *testing.T) {
	t.Parallel()
	finding := check.Finding{
		CheckID: "image-checksum",
		Kind:    "Deployment",
		Name:    "intel-gpu-monitor",
		Value:   "registry.example.com/intel-gpu-monitor:not_latest",
		File:    "kubernetes/intel-gpu-monitor/overlays/okd",
	}
	ctx := &complianceAttributionCtx{changedKeys: map[string][]string{}}

	blocking, nonblocking := classifyResourceCompliance([]check.Finding{finding}, ctx)
	if len(blocking["image-checksum"]) != 0 || len(nonblocking["image-checksum"]) != 1 {
		t.Errorf("expected an unchanged resource to be a non-blocking warning, got blocking=%d warning=%d",
			len(blocking["image-checksum"]), len(nonblocking["image-checksum"]))
	}
}

// TestIsResourceAffected_BaseResourceViaDirectlyChangedOverlay is the exact
// HomeLab PR #611 scenario: a NEW app adds base/deployment.yaml AND its
// overlays/okd/kustomization.yaml (which refs ../../base). The overlay is thus
// a "direct" overlay, but the Deployment lives in base/. The direct branch must
// NOT short-circuit - the resource must still be reached via the base/component
// branch (overlaysByDir), so the finding is blocking, not pre-existing.
func TestIsResourceAffected_BaseResourceViaDirectlyChangedOverlay(t *testing.T) {
	d := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(d); err != nil {
		t.Fatal(err)
	}

	app := "kubernetes/app"
	writeFile(t, d, app+"/base/deployment.yaml",
		"kind: Deployment\nmetadata:\n  name: app\n")
	writeFile(t, d, app+"/base/kustomization.yaml",
		"resources:\n  - deployment.yaml\n")
	writeFile(t, d, app+"/overlays/okd/kustomization.yaml",
		"resources:\n  - ../../base\n")

	changed := []string{
		app + "/base/deployment.yaml",
		app + "/base/kustomization.yaml",
		app + "/overlays/okd/kustomization.yaml",
	}
	ctx := buildAttributionCtx(changed, []string{app})

	if !ctx.directOverlays[app+"/okd"] {
		t.Fatalf("expected the overlay to be a direct overlay, got %v", ctx.directOverlays)
	}
	if got := ctx.overlaysByDir[app+"/base"]; !got[app+"/okd"] {
		t.Fatalf("expected base dir to map to the overlay (my overlaysByDir fix), got %v", ctx.overlaysByDir)
	}
	if !isResourceAffected("Deployment/app", ctx, app+"/overlays/okd") {
		t.Errorf("expected the base-defined resource to be affected (blocking) even though " +
			"the overlay is a direct overlay - the direct branch must not short-circuit")
	}
}

// TestIsFileInOverlay_MultiSegmentAndTemplates covers isFileInOverlay for both
// the multi-segment app case and the scaffold-template (templates/<app>) case.
func TestIsFileInOverlay_MultiSegmentAndTemplates(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		file    string
		app     string
		cluster string
		want    bool
	}{
		{"multi-segment match", "kubernetes/app/overlays/okd/x.yaml", "kubernetes/app", "okd", true},
		{"multi-segment wrong cluster", "kubernetes/app/overlays/okd/x.yaml", "kubernetes/app", "dev", false},
		{"multi-segment wrong app", "kubernetes/other/overlays/okd/x.yaml", "kubernetes/app", "okd", false},
		{"base file not in overlay", "kubernetes/app/base/x.yaml", "kubernetes/app", "okd", false},
		{"templates suffix match", "repo/templates/app/overlays/okd/x.yaml", "app", "okd", true},
	}
	for _, c := range cases {
		if got := isFileInOverlay(c.file, c.app, c.cluster); got != c.want {
			t.Errorf("%s: isFileInOverlay(%q,%q,%q) = %v, want %v",
				c.name, c.file, c.app, c.cluster, got, c.want)
		}
	}
}
