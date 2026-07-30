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

func TestValidateReader_OpenShiftGroups_RequireAnnotationByDefault(t *testing.T) {
	t.Cleanup(func() { AssumeOpenShift = false })
	AssumeOpenShift = false
	cases := []struct {
		name string
		data string
	}{
		{"GatewayClass", "kind: GatewayClass\napiVersion: gateway.networking.k8s.io/v1\nmetadata:\n  name: openshift-default\n"},
		{"ImageRegistryConfig", "kind: Config\napiVersion: imageregistry.operator.openshift.io/v1\nmetadata:\n  name: cluster\n"},
		{"BareMetalHost", "kind: BareMetalHost\napiVersion: metal3.io/v1alpha1\nmetadata:\n  name: worker-1\n"},
	}
	for _, c := range cases {
		errs := ValidateReader(strings.NewReader(c.data), "x.yaml")
		if len(errs) != 1 {
			t.Errorf("%s: expected 1 error with AssumeOpenShift=false: %v", c.name, errs)
		}
	}
}

func TestValidateReader_OpenShiftGroups_ExemptWhenAssumed(t *testing.T) {
	t.Cleanup(func() { AssumeOpenShift = false })
	AssumeOpenShift = true
	cases := []struct {
		name string
		data string
	}{
		{"GatewayClass", "kind: GatewayClass\napiVersion: gateway.networking.k8s.io/v1\nmetadata:\n  name: openshift-default\n"},
		{"ImageRegistryConfig", "kind: Config\napiVersion: imageregistry.operator.openshift.io/v1\nmetadata:\n  name: cluster\n"},
		{"BareMetalHost", "kind: BareMetalHost\napiVersion: metal3.io/v1alpha1\nmetadata:\n  name: worker-1\n"},
	}
	for _, c := range cases {
		errs := ValidateReader(strings.NewReader(c.data), "x.yaml")
		if len(errs) != 0 {
			t.Errorf("%s: expected no errors with AssumeOpenShift=true: %v", c.name, errs)
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
