package kubernetes

import (
	"strings"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/exempt"
)

// TestEveryValidationPackageRegisters guards a class of silent-failure bug
// that shipped undetected: a validation sub-package's checks only reach the
// registry as a side effect of this package's blank imports plus that
// package having an init(). Both halves have gone missing before -
// autoscaling, scheduling and apiextensions were never imported here, and
// batch was imported but had no init() at all, so ~30 checks were dead code
// whose own unit tests still passed happily in isolation.
//
// One representative check ID per sub-package is asserted, so adding a new
// API-group package without wiring it up fails here rather than silently
// validating nothing.
func TestEveryValidationPackageRegisters(t *testing.T) {
	tests := []struct {
		pkg string
		id  string
	}{
		{pkg: "admissionregistration", id: "admissionregistration/failure-policy-invalid"},
		{pkg: "apiextensions", id: "apiextensions/crd-storage-version-invalid"},
		{pkg: "apps", id: "apps/daemonset-min-ready-seconds-invalid"},
		{pkg: "autoscaling", id: "autoscaling/max-replicas-invalid"},
		{pkg: "batch", id: "batch/backoff-limit-invalid"},
		{pkg: "core", id: "container/duplicate-container-names"},
		{pkg: "networking", id: "ingress/path-type-invalid"},
		{pkg: "policy", id: "policy/max-unavailable-invalid"},
		{pkg: "rbac", id: "rbac/clusterrole-ref-invalid"},
		{pkg: "storage", id: "persistent-volume-claim/access-modes-invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.pkg, func(t *testing.T) {
			c, ok := check.ByID(tt.id)
			if !ok {
				t.Fatalf("%s/validation is not registered: check %q missing from the registry. "+
					"Verify register.go blank-imports the package AND the package has an init() that calls its Register().",
					tt.pkg, tt.id)
			}
			if got := c.Section(); got != "runtime-validation" {
				t.Errorf("check %q Section() = %q, want %q", tt.id, got, "runtime-validation")
			}
		})
	}
}

// TestRuntimeChecksAreNonExemptable pins the contract stated in
// pkg/validator/runtime: these findings describe manifests the API server
// itself rejects, so an exemption would only defer the failure to apply
// time. Previously check.Register marked every runtime rule ID exemptable.
func TestRuntimeChecksAreNonExemptable(t *testing.T) {
	var offenders []string
	for _, c := range check.All() {
		if c.Section() != "runtime-validation" {
			continue
		}
		if exempt.Known(c.ID()) {
			offenders = append(offenders, c.ID())
		}
	}
	if len(offenders) > 0 {
		t.Errorf("runtime checks must not be exemptable, but %d are: %s",
			len(offenders), strings.Join(offenders, ", "))
	}
}

// TestRuntimeChecksDeclareKinds ensures every runtime check filters by kind.
// A check with no Kinds() is handed every document in the changeset, which
// is both wasted work and a sign the author forgot to scope it.
func TestRuntimeChecksDeclareKinds(t *testing.T) {
	for _, c := range check.All() {
		if c.Section() != "runtime-validation" {
			continue
		}
		skipper, ok := c.(check.DocSkipper)
		if !ok {
			t.Errorf("check %q does not implement check.DocSkipper", c.ID())
			continue
		}
		// A check that skips nothing at all declared no kinds.
		if !skipper.SkipDoc("ThisKindDoesNotExist") {
			t.Errorf("check %q declares no Kinds(), so it runs against every document", c.ID())
		}
	}
}
