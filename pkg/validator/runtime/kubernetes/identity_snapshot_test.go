package kubernetes

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
)

// updateChecks regenerates the golden file instead of asserting against it.
var updateChecks = flag.Bool("update-checks", false, "regenerate testdata/registered_checks.json")

// checkIdentity is everything about a registered check that is observable
// outside this package: the ID findings are keyed by, the title rendered in
// reports, the kinds it is dispatched for, and the contract flags.
type checkIdentity struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Section         string   `json:"section"`
	Blocking        bool     `json:"blocking"`
	RenderSensitive bool     `json:"renderSensitive"`
	NonExemptable   bool     `json:"nonExemptable"`
	SkipsKinds      []string `json:"appliesTo"`
}

// probeKinds is the set of kinds SkipDoc is interrogated with to reconstruct
// a check's applies-to list without depending on how it is stored. It covers
// every kind any registered check declares plus a control that no check
// should claim.
var probeKinds = []string{
	"Pod", "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet",
	"ReplicationController", "Job", "CronJob",
	"Service", "Ingress", "IngressClass", "NetworkPolicy",
	"ConfigMap", "Secret", "Namespace", "ResourceQuota", "LimitRange",
	"PersistentVolume", "PersistentVolumeClaim", "StorageClass",
	"Role", "ClusterRole", "RoleBinding", "ClusterRoleBinding",
	"HorizontalPodAutoscaler", "PodDisruptionBudget",
	"ValidatingWebhookConfiguration", "MutatingWebhookConfiguration",
	"CustomResourceDefinition", "ServiceAccount", "Endpoints",
	"NotAKubernetesKindControl",
}

// kindsOf returns the kinds a check is dispatched for, empty meaning all.
func kindsOf(c check.Check) []string {
	k, ok := c.(interface{ Kinds() []string })
	if !ok {
		return nil
	}
	return k.Kinds()
}

func collectCheckIdentities(t *testing.T) []checkIdentity {
	t.Helper()

	var out []checkIdentity
	for _, c := range check.All() {
		if c.Section() != "runtime-validation" {
			continue
		}
		id := checkIdentity{
			ID:       c.ID(),
			Title:    c.Title(),
			Section:  c.Section(),
			Blocking: c.Blocking(),
		}
		if rs, ok := c.(interface{ RenderSensitive() bool }); ok {
			id.RenderSensitive = rs.RenderSensitive()
		}
		if ne, ok := c.(interface{ NonExemptable() bool }); ok {
			id.NonExemptable = ne.NonExemptable()
		}
		// Reconstruct applies-to through the real dispatch path rather than
		// reading a field, so the snapshot stays valid across any change to
		// how kinds are declared.
		if sd, ok := c.(interface{ SkipDoc(string) bool }); ok {
			for _, k := range probeKinds {
				if !sd.SkipDoc(k) {
					id.SkipsKinds = append(id.SkipsKinds, k)
				}
			}
		}
		sort.Strings(id.SkipsKinds)
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// TestRegisteredCheckIdentityIsStable pins the externally-visible identity of
// every runtime check against a golden file.
//
// Findings are keyed by check ID, reports group and title by it, and the
// dispatcher decides what a check ever sees from its kind list. A refactor
// that renames an ID, drops a kind, loses a check or flips a contract flag
// is not a refactor - it silently changes what CI enforces, in a family that
// is non-exemptable and therefore has no suppression audit trail to notice
// it. None of that is visible in a diff that only moves code around.
//
// Regenerate deliberately with: go test ./pkg/validator/runtime/kubernetes/ -update-checks
func TestRegisteredCheckIdentityIsStable(t *testing.T) {
	got := collectCheckIdentities(t)
	if len(got) == 0 {
		t.Fatal("no runtime checks registered; the snapshot would be vacuous")
	}

	golden := filepath.Join("testdata", "registered_checks.json")

	encoded, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatalf("encoding identities: %v", err)
	}
	encoded = append(encoded, '\n')

	if *updateChecks {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("creating testdata: %v", err)
		}
		if err := os.WriteFile(golden, encoded, 0o644); err != nil {
			t.Fatalf("writing golden file: %v", err)
		}
		t.Logf("wrote %s with %d checks", golden, len(got))
		return
	}

	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("reading %s (regenerate with -update-checks): %v", golden, err)
	}

	if string(encoded) == string(want) {
		return
	}

	// Report the difference as check-level changes rather than a raw text
	// diff, so the failure names what actually moved.
	var wantIDs []checkIdentity
	if err := json.Unmarshal(want, &wantIDs); err != nil {
		t.Fatalf("parsing golden file: %v", err)
	}

	wantByID := make(map[string]checkIdentity, len(wantIDs))
	for _, c := range wantIDs {
		wantByID[c.ID] = c
	}
	gotByID := make(map[string]checkIdentity, len(got))
	for _, c := range got {
		gotByID[c.ID] = c
	}

	for id := range wantByID {
		if _, ok := gotByID[id]; !ok {
			t.Errorf("check %q disappeared from the registry", id)
		}
	}
	for id, g := range gotByID {
		w, ok := wantByID[id]
		if !ok {
			t.Errorf("check %q is new; regenerate the golden file if intended", id)
			continue
		}
		if g.Title != w.Title {
			t.Errorf("check %q title changed: %q -> %q", id, w.Title, g.Title)
		}
		if g.Blocking != w.Blocking || g.RenderSensitive != w.RenderSensitive || g.NonExemptable != w.NonExemptable {
			t.Errorf("check %q contract flags changed: blocking %v->%v, renderSensitive %v->%v, nonExemptable %v->%v",
				id, w.Blocking, g.Blocking, w.RenderSensitive, g.RenderSensitive, w.NonExemptable, g.NonExemptable)
		}
		if strings.Join(g.SkipsKinds, ",") != strings.Join(w.SkipsKinds, ",") {
			t.Errorf("check %q applies-to changed:\n  was: %v\n  now: %v", id, w.SkipsKinds, g.SkipsKinds)
		}
	}

	if len(got) != len(wantIDs) {
		t.Errorf("check count changed: %d -> %d", len(wantIDs), len(got))
	}
}

// TestChecksIgnoreKindsTheyDoNotDeclare asserts the applies-to contract for
// every registered check at once: a check handed a kind outside its list
// must be skipped by the dispatcher and, if run anyway, must report nothing.
//
// This replaces a per-package family of near-identical "NonMatchingKind"
// tests, each of which sampled a single check. Driving it from the registry
// covers all of them, including any check added later - which is the case
// the hand-written versions could never cover.
func TestChecksIgnoreKindsTheyDoNotDeclare(t *testing.T) {
	// A syntactically valid document of a kind no runtime check declares.
	const foreign = "apiVersion: example.com/v1\nkind: NotAKubernetesKindControl\nmetadata:\n  name: test\n"

	for _, c := range check.All() {
		if c.Section() != "runtime-validation" {
			continue
		}
		t.Run(c.ID(), func(t *testing.T) {
			sd, ok := c.(interface{ SkipDoc(string) bool })
			if !ok {
				t.Fatalf("check %q does not implement SkipDoc, so the dispatcher hands it every document", c.ID())
			}
			// A check that declares no kinds applies to all of them
			// (object metadata rules), so it is expected not to skip.
			// Its Run must still find nothing on an unrelated document.
			if !sd.SkipDoc("NotAKubernetesKindControl") && len(kindsOf(c)) != 0 {
				t.Errorf("check %q claims a kind it has no rules for", c.ID())
			}
			dc, ok := c.(interface {
				CheckDoc(data []byte, source string) []check.Finding
			})
			if !ok {
				t.Fatalf("check %q is not a DocCheck", c.ID())
			}
			if got := dc.CheckDoc([]byte(foreign), "test.yaml"); len(got) != 0 {
				t.Errorf("check %q reported %d finding(s) on an unrelated kind: %v", c.ID(), len(got), got)
			}
		})
	}
}
