package validator

import (
	"os"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
)

// TestResourceKeyFor_NamespaceAware proves two resources with the same Kind and
// Name but different namespaces produce distinct attribution keys, while a
// cluster-scoped (empty-namespace) resource collapses to the historical
// Kind/Name form so the 9 namespace-blind checks are unaffected.
func TestResourceKeyFor_NamespaceAware(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		namespace string
		kind      string
		resName   string
		want      string
	}{
		{"namespaced", "stackrox", "Certificate", "central-default-tls-cert", "stackrox/Certificate/central-default-tls-cert"},
		{"other ns", "stackrox-tekton", "Certificate", "central-default-tls-cert", "stackrox-tekton/Certificate/central-default-tls-cert"},
		{"cluster-scoped degrades", "", "ClusterRole", "cr", "ClusterRole/cr"},
	}
	for _, c := range cases {
		if got := resourceKeyFor(c.namespace, c.kind, c.resName); got != c.want {
			t.Errorf("%s: resourceKeyFor(%q,%q,%q) = %q, want %q",
				c.name, c.namespace, c.kind, c.resName, got, c.want)
		}
	}
}

// TestChangedResourceKeys_NamespaceAware reproduces the attribution bug where a
// rendered finding missing the sync-options annotation was pointed at the wrong
// source file: two manifests can declare the same Kind/Name but live in
// different namespaces, and only one of them was changed in the PR. Namely,
// changedResourceKeys must key each source file by its namespace-qualified
// identity so that a finding for one namespace resolves to that namespace's
// file - not an unrelated, co-named file that happens to have been touched.
func TestChangedResourceKeys_NamespaceAware(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	app := "acs"
	// Two source files, same Certificate name, different namespaces. Only the
	// "stackrox" copy was changed in the PR (mimicking a PR that fixed that one
	// but left the co-named "stackrox-tekton" copy outstanding).
	stackroxFile := writeFile(t, d, app+"/components/a/certificate.yaml",
		"kind: Certificate\nmetadata:\n  name: central-default-tls-cert\n  namespace: stackrox\n")
	writeFile(t, d, app+"/components/b/certificate.yaml",
		"kind: Certificate\nmetadata:\n  name: central-default-tls-cert\n  namespace: stackrox-tekton\n")

	changed := []string{stackroxFile}
	keys := changedResourceKeys(changed)

	// The changed stackrox file must be reachable via its namespace-aware key so
	// a sync-options finding (which populates Namespace) resolves to it.
	if snap := keys["stackrox/Certificate/central-default-tls-cert"]; len(snap) != 1 || snap[0] != stackroxFile {
		t.Errorf("expected changedResourceKeys to map the stackrox copy under the namespace key: got %v", snap)
	}

	// The untouched co-named file must NOT be registered under the unchanged
	// namespace key, or a finding for "stackrox-tekton" would incorrectly pull
	// in an unrelated, changed co-named file.
	if files := keys["stackrox-tekton/Certificate/central-default-tls-cert"]; len(files) != 0 {
		t.Errorf("expected no source file under the unchanged stackrox-tekton namespace key, got %v", files)
	}

	// The legacy namespace-blind key must also resolve to the changed stackrox
	// file, so namespace-blind checks (those not populating Namespace) still
	// classify their directly-changed resources as blocking.
	if snap := keys["Certificate/central-default-tls-cert"]; len(snap) != 1 || snap[0] != stackroxFile {
		t.Errorf("expected changedResourceKeys to map the stackrox copy under the legacy Kind/Name key: got %v", snap)
	}
}

// TestChangedResourceKeys_ClusterScopedNoDuplicate is the regression for a bug
// where the dual-write of a cluster-scoped source (empty namespace) collapsed
// to the same Kind/Name key twice, duplicating the file in the slice under that
// key. A cluster-scoped resource must be listed exactly once.
func TestChangedResourceKeys_ClusterScopedNoDuplicate(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	app := "myapp"
	cr := writeFile(t, d, app+"/base/clusterrole.yaml",
		"kind: ClusterRole\nmetadata:\n  name: cr\n") // no namespace → cluster-scoped

	keys := changedResourceKeys([]string{cr})

	if got := keys["ClusterRole/cr"]; len(got) != 1 || got[0] != cr {
		t.Errorf("expected exactly one source entry under the legacy Kind/Name key for a cluster-scoped resource, got %v", got)
	}
}

// TestClassifyResourceCompliance_NamespacedSourceStillBlockingForLegacyCheck is
// the regression for a bug where indexing changed sources by namespace only
// would silently downgrade direct findings for the namespace-blind checks
// (those that don't populate Finding.Namespace) from blocking to non-blocking.
// A changed source manifest that declares metadata.namespace must still be
// reachable by the legacy Kind/Name ResourceKey those checks use.
func TestClassifyResourceCompliance_NamespacedSourceStillBlockingForLegacyCheck(t *testing.T) {
	d := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(d); err != nil {
		t.Fatal(err)
	}

	app := "myapp"
	// The directly-changed source manifest declares a namespace, as almost every
	// real manifest does.
	writeFile(t, d, app+"/base/deployment.yaml",
		"kind: Deployment\nmetadata:\n  name: api\n  namespace: team-a\n")
	writeFile(t, d, app+"/base/kustomization.yaml", "resources:\n  - deployment.yaml\n")
	writeFile(t, d, app+"/overlays/dev/kustomization.yaml", "resources:\n  - ../../base\n")

	changed := []string{
		app + "/base/deployment.yaml",
		app + "/base/kustomization.yaml",
		app + "/overlays/dev/kustomization.yaml",
	}
	ctx := buildAttributionCtx(changed, []string{app})

	// A namespace-blind check (Namespace empty, e.g. image-checksum) finding the
	// changed namespaced resource must still be blocking.
	finding := check.Finding{
		CheckID: "image-checksum",
		Kind:    "Deployment",
		Name:    "api",
		// Namespace deliberately unset: namespace-blind legacy behavior.
		Value: "registry.example.com/api:not_pinned",
		File:  app + "/overlays/dev",
	}

	blocking, nonblocking := classifyResourceCompliance([]check.Finding{finding}, ctx)
	if len(blocking["image-checksum"]) != 1 || len(nonblocking["image-checksum"]) != 0 {
		t.Errorf("expected the directly-changed namespaced resource to stay blocking for a namespace-blind check, got blocking=%d warning=%d",
			len(blocking["image-checksum"]), len(nonblocking["image-checksum"]))
	}
}

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
