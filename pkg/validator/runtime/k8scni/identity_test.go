package k8scni

import (
	"sort"
	"strings"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// family is the upstream family every check in this package belongs to.
const family = "k8scni"

// TestRegisteredCheckIdentityIsStable pins the externally-visible identity of
// this family's checks: the ID findings are keyed by, the title reports use,
// and the kinds the dispatcher will ever hand them.
//
// The sibling kubernetes family pins the same three against a golden file,
// because 87 checks are not readable inline. Two are, so this asserts them
// directly rather than adding a second golden file and a second regeneration
// path to keep in sync - the protection is identical, the machinery is not.
//
// A change here is not a refactor. It changes what CI enforces, and in a
// non-exemptable family there is no suppression audit trail where that would
// otherwise surface.
func TestRegisteredCheckIdentityIsStable(t *testing.T) {
	Register()

	want := map[string]struct {
		title string
		kinds string
	}{
		"k8scni/net-attach-def/config-invalid": {
			title: "NetworkAttachmentDefinition spec.config Must Be A Valid CNI Configuration",
			kinds: "NetworkAttachmentDefinition",
		},
		"k8scni/net-attach-def/ovn-netconf-invalid": {
			title: "OVN-Kubernetes NetworkAttachmentDefinition Semantic Rules",
			kinds: "NetworkAttachmentDefinition",
		},
	}

	got := map[string]bool{}
	for _, c := range check.All() {
		if c.Section() != "runtime-validation" || runtime.FamilyOf(c.ID()) != family {
			continue
		}
		got[c.ID()] = true

		w, ok := want[c.ID()]
		if !ok {
			t.Errorf("unexpected %s check %q; add it here deliberately", family, c.ID())
			continue
		}
		if c.Title() != w.title {
			t.Errorf("check %q title = %q, want %q", c.ID(), c.Title(), w.title)
		}
		if kinds := appliesTo(t, c); kinds != w.kinds {
			t.Errorf("check %q applies to %q, want %q", c.ID(), kinds, w.kinds)
		}
	}

	for id := range want {
		if !got[id] {
			t.Errorf("check %q is not registered; its findings would never fire", id)
		}
	}
}

// appliesTo reconstructs the kinds a check is dispatched for.
//
// The registry hands back the adapter, which exposes only check.DocSkipper
// and not the raw kind slice, so this has to be reconstructed through the
// same call the dispatcher makes. The control kind is one no check may claim:
// if it is not skipped, the check applies to everything.
func appliesTo(t *testing.T, c check.Check) string {
	t.Helper()

	const controlKind = "NotAKubernetesKindControl"
	probe := []string{"NetworkAttachmentDefinition", "Pod", "Deployment", "ConfigMap", controlKind}

	sd, ok := c.(interface{ SkipDoc(string) bool })
	if !ok {
		return "*"
	}
	var out []string
	for _, k := range probe {
		if !sd.SkipDoc(k) {
			out = append(out, k)
		}
	}
	if len(out) == len(probe) {
		return "*"
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}
