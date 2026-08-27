package syncopts

import (
	"strings"
	"testing"
)

func TestValidateReader_Builtin(t *testing.T) {
	data := `kind: Deployment
apiVersion: apps/v1
metadata:
  name: d
`
	errs := ValidateReader(strings.NewReader(data), "x.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors for builtin: %v", errs)
	}
}

func TestValidateReader_CRD_NoAnnotation(t *testing.T) {
	data := `kind: ArgoCD
apiVersion: argoproj.io/v1alpha1
metadata:
  name: cd
`
	errs := ValidateReader(strings.NewReader(data), "x.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error: %v", errs)
	}
}

func TestValidateReader_KustomizeComponent(t *testing.T) {
	data := `kind: Component
apiVersion: kustomize.config.k8s.io/v1alpha1
resources:
  - deployment.yaml
`
	errs := ValidateReader(strings.NewReader(data), "x.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors for kustomize Component: %v", errs)
	}
}

func TestValidateReader_KustomizeKustomization(t *testing.T) {
	data := `kind: Kustomization
apiVersion: kustomize.config.k8s.io/v1beta1
resources:
  - deployment.yaml
`
	errs := ValidateReader(strings.NewReader(data), "x.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors for kustomize Kustomization: %v", errs)
	}
}

func TestValidateReader_CRD_Annotation(t *testing.T) {
	data := `kind: ArgoCD
apiVersion: argoproj.io/v1alpha1
metadata:
  name: cd
  annotations:
    argocd.argoproj.io/sync-options: SkipDryRunOnMissingResource=true
`
	errs := ValidateReader(strings.NewReader(data), "x.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors with annotation: %v", errs)
	}
}

func TestValidateReader_OpenShiftDefaultGroups_RequireAnnotationByDefault(t *testing.T) {
	t.Cleanup(func() { AssumeOpenShift = false })
	AssumeOpenShift = false
	cases := []struct {
		name string
		data string
	}{
		{"GatewayClass", "kind: GatewayClass\napiVersion: gateway.networking.k8s.io/v1\nmetadata:\n  name: openshift-default\n"},
		{"PrometheusRule", "kind: PrometheusRule\napiVersion: monitoring.coreos.com/v1\nmetadata:\n  name: rules\n"},
		{"BareMetalHost", "kind: BareMetalHost\napiVersion: metal3.io/v1alpha1\nmetadata:\n  name: worker-1\n"},
	}
	for _, c := range cases {
		errs := ValidateReader(strings.NewReader(c.data), "x.yaml")
		if len(errs) != 1 {
			t.Errorf("%s: expected 1 error with AssumeOpenShift=false: %v", c.name, errs)
		}
	}
}

func TestValidateReader_OpenShiftDefaultGroups_ExemptWhenAssumed(t *testing.T) {
	t.Cleanup(func() { AssumeOpenShift = false })
	AssumeOpenShift = true
	cases := []struct {
		name string
		data string
	}{
		{"GatewayClass", "kind: GatewayClass\napiVersion: gateway.networking.k8s.io/v1\nmetadata:\n  name: openshift-default\n"},
		{"PrometheusRule", "kind: PrometheusRule\napiVersion: monitoring.coreos.com/v1\nmetadata:\n  name: rules\n"},
		{"BareMetalHost", "kind: BareMetalHost\napiVersion: metal3.io/v1alpha1\nmetadata:\n  name: worker-1\n"},
	}
	for _, c := range cases {
		errs := ValidateReader(strings.NewReader(c.data), "x.yaml")
		if len(errs) != 0 {
			t.Errorf("%s: expected no errors with AssumeOpenShift=true: %v", c.name, errs)
		}
	}
}

func TestValidateReader_OpenShiftExclusiveGroups_AlwaysExempt(t *testing.T) {
	t.Cleanup(func() { AssumeOpenShift = false })
	cases := []struct {
		name string
		data string
	}{
		{"ImageRegistryConfig", "kind: Config\napiVersion: imageregistry.operator.openshift.io/v1\nmetadata:\n  name: cluster\n"},
		{"Route", "kind: Route\napiVersion: route.openshift.io/v1\nmetadata:\n  name: my-route\n"},
	}
	for _, assume := range []bool{false, true} {
		AssumeOpenShift = assume
		for _, c := range cases {
			errs := ValidateReader(strings.NewReader(c.data), "x.yaml")
			if len(errs) != 0 {
				t.Errorf("%s (AssumeOpenShift=%v): expected no errors, openshift-exclusive groups are always exempt: %v", c.name, assume, errs)
			}
		}
	}
}

func TestValidateReader_NonExemptGroups_AlwaysRequireAnnotation(t *testing.T) {
	for _, assume := range []bool{false, true} {
		AssumeOpenShift = assume
		cases := []struct {
			name string
			data string
		}{
			{"Certificate", "kind: Certificate\napiVersion: cert-manager.io/v1\nmetadata:\n  name: cert\n"},
			{"ServiceEntry", "kind: ServiceEntry\napiVersion: networking.istio.io/v1\nmetadata:\n  name: se\n"},
			{"AuthorizationPolicy", "kind: AuthorizationPolicy\napiVersion: security.istio.io/v1\nmetadata:\n  name: ap\n"},
		}
		for _, c := range cases {
			errs := ValidateReader(strings.NewReader(c.data), "x.yaml")
			if len(errs) != 1 {
				t.Errorf("%s (AssumeOpenShift=%v): expected 1 error: %v", c.name, assume, errs)
			}
		}
	}
	AssumeOpenShift = false
}

func TestValidateReader_InstallerOnlyKinds(t *testing.T) {
	for _, assume := range []bool{false, true} {
		AssumeOpenShift = assume
		cases := []struct {
			name string
			data string
		}{
			{"AgentConfig", "kind: AgentConfig\napiVersion: v1beta1\nmetadata:\n  name: okd-test\n"},
			{"InstallConfig", "kind: InstallConfig\napiVersion: v1beta1\nmetadata:\n  name: okd\n"},
		}
		for _, c := range cases {
			errs := ValidateReader(strings.NewReader(c.data), "x.yaml")
			if len(errs) != 0 {
				t.Errorf("%s (AssumeOpenShift=%v): expected no errors: %v", c.name, assume, errs)
			}
		}
	}
	AssumeOpenShift = false
}

func TestDeduplicatedError_String(t *testing.T) {
	d := DeduplicatedError{APIVersion: "argoproj.io/v1alpha1", Kind: "ArgoCD", Name: "cd", Count: 2}
	if !strings.Contains(d.String(), "missing argocd.argoproj.io/sync-options") {
		t.Errorf("unexpected string: %q", d.String())
	}
}

func TestExtractGroup(t *testing.T) {
	cases := map[string]string{
		"v1": "", "apps/v1": "apps", "argoproj.io/v1alpha1": "argoproj.io",
	}
	for in, want := range cases {
		if got := extractGroup(in); got != want {
			t.Errorf("extractGroup(%q) = %q", in, got)
		}
	}
}

// --- IsBuiltinResource table test -------------------------------------
//
// One row per group from the PR-4 tiering table, plus a sampling of
// pre-existing groups - this also regression-tests the
// "ingressoperator.openshift.io" -> "ingress.operator.openshift.io" typo
// fix (a resource in the corrected group is now properly exempt under
// AssumeOpenShift=true, whereas before the fix the typo'd key never
// matched a real API group).

func TestIsBuiltinResource(t *testing.T) {
	cases := []struct {
		apiVersion     string
		assumeOS       bool
		wantBuiltin    bool
		nameForFailure string
	}{
		// coreAPIGroups - always exempt regardless of AssumeOpenShift.
		{"cli.kyverno.io/v1alpha1", false, true, "cli.kyverno.io (core)"},
		{"policy.networking.k8s.io/v1alpha1", false, true, "policy.networking.k8s.io (core)"},
		{"populator.storage.k8s.io/v1alpha1", false, true, "populator.storage.k8s.io (core)"},
		{"snapshot.storage.k8s.io/v1", false, true, "snapshot.storage.k8s.io (core)"},
		{"internal.apiserver.k8s.io/v1alpha1", false, true, "internal.apiserver.k8s.io (core)"},
		{"resource.k8s.io/v1alpha3", false, true, "resource.k8s.io (core)"},
		{"storagemigration.k8s.io/v1alpha1", false, true, "storagemigration.k8s.io (core)"},
		{"infrastructure.cluster.x-k8s.io/v1beta1", false, true, "infrastructure.cluster.x-k8s.io (core)"},
		{"ipam.cluster.x-k8s.io/v1beta1", false, true, "ipam.cluster.x-k8s.io (core)"},

		// openshiftExclusiveAPIGroups - always exempt, regardless of
		// AssumeOpenShift, since these groups can only ever exist on an
		// OpenShift/OKD API server.
		{"route.openshift.io/v1", false, true, "route.openshift.io (exclusive, not assumed)"},
		{"route.openshift.io/v1", true, true, "route.openshift.io (exclusive, assumed)"},
		{"imageregistry.operator.openshift.io/v1", false, true, "imageregistry.operator.openshift.io (exclusive, not assumed)"},

		// openshiftDefaultAPIGroups - only exempt when AssumeOpenShift=true,
		// since these groups also ship on non-OpenShift Kubernetes clusters.
		{"k8s.ovn.org/v1", false, false, "k8s.ovn.org (not assumed)"},
		{"k8s.ovn.org/v1", true, true, "k8s.ovn.org (assumed)"},
		{"monitoring.coreos.com/v1", false, false, "monitoring.coreos.com (not assumed)"},
		{"monitoring.coreos.com/v1", true, true, "monitoring.coreos.com (assumed)"},
		{"operatorhub.io/v1alpha1", true, true, "operatorhub.io"},
		{"hive.openshift.io/v1", false, false, "hive.openshift.io (not assumed)"},
		{"hive.openshift.io/v1", true, true, "hive.openshift.io (assumed)"},

		// openshiftExclusiveAPIGroups - always exempt (moved out of the
		// former flag-gated map).
		{"network.openshift.io/v1", false, true, "network.openshift.io"},
		{"network.operator.openshift.io/v1", false, true, "network.operator.openshift.io"},
		{"cloud.network.openshift.io/v1", false, true, "cloud.network.openshift.io"},
		{"build.openshift.io/v1", false, true, "build.openshift.io"},
		{"apps.openshift.io/v1", false, true, "apps.openshift.io"},
		{"template.openshift.io/v1", false, true, "template.openshift.io"},
		{"authorization.openshift.io/v1", false, true, "authorization.openshift.io"},
		{"user.openshift.io/v1", false, true, "user.openshift.io"},
		{"oauth.openshift.io/v1", false, true, "oauth.openshift.io"},
		{"security.internal.openshift.io/v1", false, true, "security.internal.openshift.io"},
		{"monitoring.openshift.io/v1", false, true, "monitoring.openshift.io"},
		{"cloudcredential.openshift.io/v1", false, true, "cloudcredential.openshift.io"},
		{"performance.openshift.io/v2", false, true, "performance.openshift.io"},
		{"apiserver.openshift.io/v1", false, true, "apiserver.openshift.io"},
		{"autoscaling.openshift.io/v1", false, true, "autoscaling.openshift.io"},

		// Regression for the typo fix: the corrected group name is now
		// recognized as an always-exempt exclusive group.
		{"ingress.operator.openshift.io/v1", false, true, "ingress.operator.openshift.io (typo fix)"},
		// The old, typo'd group name must NOT match anything (it's not a
		// real API group and shouldn't accidentally be treated as builtin).
		{"ingressoperator.openshift.io/v1", true, false, "old typo'd group must not match"},
	}
	for _, c := range cases {
		t.Run(c.nameForFailure, func(t *testing.T) {
			old := AssumeOpenShift
			AssumeOpenShift = c.assumeOS
			defer func() { AssumeOpenShift = old }()
			if got := isBuiltinResource(c.apiVersion); got != c.wantBuiltin {
				t.Errorf("isBuiltinResource(%q) with AssumeOpenShift=%v = %v, want %v", c.apiVersion, c.assumeOS, got, c.wantBuiltin)
			}
		})
	}
}

func TestHasSkipDryRun(t *testing.T) {
	if hasSkipDryRun(map[string]string{}) {
		t.Error("expected false for no annotations")
	}
	if !hasSkipDryRun(map[string]string{RequiredAnnotation: RequiredValue}) {
		t.Error("expected true for exact match")
	}
	if !hasSkipDryRun(map[string]string{RequiredAnnotation: "Prune=false," + RequiredValue}) {
		t.Error("expected true for comma-separated value containing the required value")
	}
	if hasSkipDryRun(map[string]string{"unrelated": RequiredValue}) {
		t.Error("expected false when the value is under the wrong annotation key")
	}
}

func TestDeduplicate(t *testing.T) {
	errs := []ValidationError{
		{File: "a.yaml", APIVersion: "argoproj.io/v1alpha1", Kind: "ArgoCD", Name: "cd"},
		{File: "b.yaml", APIVersion: "argoproj.io/v1alpha1", Kind: "ArgoCD", Name: "cd"},
	}
	ded := Deduplicate(errs)
	if len(ded) != 1 || ded[0].Count != 2 {
		t.Errorf("unexpected dedup: %v", ded)
	}
}

func TestFormatComment(t *testing.T) {
	d := DeduplicatedError{APIVersion: "argoproj.io/v1alpha1", Kind: "ArgoCD", Name: "cd", Count: 1}
	s := FormatComment([]DeduplicatedError{d})
	if !strings.Contains(s, Marker) {
		t.Errorf("expected marker: %q", s)
	}
	if !strings.Contains(s, RequiredAnnotation) {
		t.Errorf("expected required annotation name: %q", s)
	}
}

func TestFormatComment_Empty(t *testing.T) {
	if s := FormatComment(nil); s != "" {
		t.Errorf("expected empty string for no findings, got: %q", s)
	}
}

// --- testdata-fixture-driven tests --------------------------------------

func TestValidateFile_KyvernoCLITest(t *testing.T) {
	// Regression for change #2: cli.kyverno.io is a local CLI-only test
	// fixture kind, never applied to a cluster.
	errs := ValidateFile("testdata/kyverno-cli-test.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors for a cli.kyverno.io Test fixture, got: %v", errs)
	}
}

func TestValidateFile_OpenShiftBuiltin(t *testing.T) {
	t.Cleanup(func() { AssumeOpenShift = false })

	// Route is an openshift-exclusive group: always exempt, regardless of
	// AssumeOpenShift, since its presence itself proves the target is
	// OpenShift/OKD.
	AssumeOpenShift = false
	errs := ValidateFile("testdata/openshift-builtin.yaml")
	if len(errs) != 0 {
		t.Errorf("expected 0 errors for a Route with AssumeOpenShift=false, got: %v", errs)
	}

	AssumeOpenShift = true
	errs = ValidateFile("testdata/openshift-builtin.yaml")
	if len(errs) != 0 {
		t.Errorf("expected 0 errors for a Route with AssumeOpenShift=true, got: %v", errs)
	}
}

func TestValidateFile_Malformed(t *testing.T) {
	// Helm template syntax must not hang or panic the YAML decoder.
	errs := ValidateFile("testdata/invalid/malformed.yaml")
	if len(errs) > 1 {
		t.Errorf("expected at most a benign result for malformed Helm-template YAML, got: %v", errs)
	}
}

func TestValidateFile_MultiDoc(t *testing.T) {
	errs := ValidateFile("testdata/multi-doc.yaml")
	if len(errs) != 2 {
		t.Fatalf("expected 2 findings (one per doc, both non-exempt CRDs), got %d: %v", len(errs), errs)
	}
}

func TestValidateFile_BadCRD(t *testing.T) {
	errs := ValidateFile("testdata/bad-crd.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %v", len(errs), errs)
	}
	if errs[0].Kind != "ArgoCD" || errs[0].Name != "my-argocd-instance" {
		t.Errorf("unexpected finding identity: %+v", errs[0])
	}
}

func TestValidateFile_CommaSeparatedAnnotationValue(t *testing.T) {
	errs := ValidateFile("testdata/comma-separated.yaml")
	if len(errs) != 0 {
		t.Errorf("expected the comma-separated annotation value to still satisfy the requirement, got: %v", errs)
	}
}

func TestValidateFile_Builtin(t *testing.T) {
	errs := ValidateFile("testdata/builtin.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors for a builtin Deployment, got: %v", errs)
	}
}

func TestValidateFile_CoreAPI(t *testing.T) {
	errs := ValidateFile("testdata/core-api.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors for a core/v1 ConfigMap, got: %v", errs)
	}
}

func TestValidateFile_MissingFile(t *testing.T) {
	errs := ValidateFile("testdata/does-not-exist.yaml")
	if errs != nil {
		t.Errorf("expected nil for a nonexistent file, got: %v", errs)
	}
}
