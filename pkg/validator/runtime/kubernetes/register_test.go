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
		{pkg: "admissionregistration", id: "kubernetes/admissionregistration/failure-policy-invalid"},
		{pkg: "apiextensions", id: "kubernetes/apiextensions/crd-storage-version-invalid"},
		{pkg: "apps", id: "kubernetes/apps/daemonset-min-ready-seconds-invalid"},
		{pkg: "autoscaling", id: "kubernetes/autoscaling/max-replicas-invalid"},
		{pkg: "batch", id: "kubernetes/batch/backoff-limit-invalid"},
		{pkg: "core", id: "kubernetes/container/duplicate-container-names"},
		{pkg: "networking", id: "kubernetes/ingress/path-type-invalid"},
		{pkg: "policy", id: "kubernetes/policy/max-unavailable-invalid"},
		{pkg: "rbac", id: "kubernetes/rbac/clusterrole-ref-invalid"},
		{pkg: "storage", id: "kubernetes/persistent-volume-claim/access-modes-invalid"},
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

// kindIndependentChecks are the checks that deliberately apply to every kind.
//
// This is an explicit allowlist rather than a relaxation of the rule below,
// so that "I forgot to scope this check" still fails for everything else. A
// check belongs here only when the upstream rule it ports is genuinely
// kind-independent, which today means the object-metadata rules the API
// server applies to every object it accepts, custom resources included.
// Enumerating kinds for those is not merely tedious but impossible, since the
// set includes CRDs this tool has never seen.
var kindIndependentChecks = map[string]string{
	"kubernetes/core/object-meta-labels-invalid":      "ValidateLabels runs on every object",
	"kubernetes/core/object-meta-annotations-invalid": "ValidateAnnotations runs on every object",
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
		unscoped := !skipper.SkipDoc("ThisKindDoesNotExist")
		_, allowed := kindIndependentChecks[c.ID()]

		if unscoped && !allowed {
			t.Errorf("check %q declares no Kinds(), so it runs against every document", c.ID())
		}
		// Keep the allowlist honest: an entry that starts scoping itself
		// should be removed rather than left to rot.
		if !unscoped && allowed {
			t.Errorf("check %q is in kindIndependentChecks but declares Kinds(); remove the allowlist entry", c.ID())
		}
	}
}
