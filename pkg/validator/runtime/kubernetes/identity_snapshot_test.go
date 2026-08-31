package kubernetes

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
)

// updateChecks regenerates the golden file instead of asserting against it.
var updateChecks = flag.Bool("update-checks", false, "regenerate testdata/registered_checks.txt")

const goldenPath = "testdata/registered_checks.txt"

// probeKinds is the universe of kinds appliesTo interrogates SkipDoc with.
//
// The registry hands back the adapter, which deliberately exposes only
// check.DocSkipper and not the raw kind slice, so the applies-to list has to
// be reconstructed through the same call the dispatcher makes. Reading a
// Kinds() method off the adapter silently matches nothing and records every
// check as applying to everything.
//
// The trailing control kind is one no check may claim.
var probeKinds = strings.Split("Pod,Deployment,StatefulSet,DaemonSet,ReplicaSet,ReplicationController,"+
	"Job,CronJob,Service,Ingress,IngressClass,NetworkPolicy,ConfigMap,Secret,Namespace,ResourceQuota,"+
	"LimitRange,PersistentVolume,PersistentVolumeClaim,StorageClass,Role,ClusterRole,RoleBinding,"+
	"ClusterRoleBinding,HorizontalPodAutoscaler,PodDisruptionBudget,ValidatingWebhookConfiguration,"+
	"MutatingWebhookConfiguration,CustomResourceDefinition,ServiceAccount,Endpoints,"+
	"NotAKubernetesKindControl", ",")

// controlKind is the last probe entry: a kind no runtime check declares.
const controlKind = "NotAKubernetesKindControl"

// appliesTo reconstructs the kinds a check is dispatched for. An empty
// result means it applies to every kind.
func appliesTo(c check.Check) []string {
	sd, ok := c.(interface{ SkipDoc(string) bool })
	if !ok {
		return nil
	}
	var out []string
	for _, k := range probeKinds {
		if !sd.SkipDoc(k) {
			out = append(out, k)
		}
	}
	// Applies to everything: it did not skip even the control kind.
	if len(out) == len(probeKinds) {
		return nil
	}
	return out
}

// checkIdentityLines renders one tab-separated line per registered runtime
// check: rule ID, title, and the kinds it applies to.
//
// Only these three vary. Section, blocking, renderSensitive and
// nonExemptable are identical for every check in the family and are
// asserted directly by TestRuntimeChecksAreNonExemptable and
// TestEveryValidationPackageRegisters, so recording them here would be 300
// lines restating what another test already proves.
func checkIdentityLines(t *testing.T) []string {
	t.Helper()

	var out []string
	for _, c := range check.All() {
		if c.Section() != "runtime-validation" {
			continue
		}
		kinds := "*"
		if ks := appliesTo(c); len(ks) > 0 {
			sort.Strings(ks)
			kinds = strings.Join(ks, ",")
		}
		out = append(out, fmt.Sprintf("%s\t%s\t%s", c.ID(), c.Title(), kinds))
	}
	sort.Strings(out)
	return out
}

// TestRegisteredCheckIdentityIsStable pins the externally-visible identity of
// every runtime check against a golden file.
//
// Findings are keyed by check ID, reports title by it, and the dispatcher
// decides what a check ever sees from its kind list. A change to any of them
// is not a refactor: it changes what CI enforces, and in a non-exemptable
// family there is no suppression audit trail where that would surface. None
// of it is visible in a diff that only moves code around.
//
// Regenerate deliberately with:
//
//	task update:checks
func TestRegisteredCheckIdentityIsStable(t *testing.T) {
	got := checkIdentityLines(t)
	if len(got) == 0 {
		t.Fatal("no runtime checks registered; the snapshot would be vacuous")
	}

	header := "# Registered runtime checks: <rule id>\\t<title>\\t<kinds, comma-separated, * = all>\n" +
		"# Regenerate: task update:checks\n"
	content := header + strings.Join(got, "\n") + "\n"

	if *updateChecks {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("creating testdata: %v", err)
		}
		if err := os.WriteFile(goldenPath, []byte(content), 0o644); err != nil {
			t.Fatalf("writing golden file: %v", err)
		}
		t.Logf("wrote %s with %d checks", goldenPath, len(got))
		return
	}

	raw, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading %s (regenerate with `task update:checks`): %v", goldenPath, err)
	}

	var want []string
	for _, line := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
		if !strings.HasPrefix(line, "#") {
			want = append(want, line)
		}
	}

	wantSet := make(map[string]string, len(want))
	for _, l := range want {
		id, rest, _ := strings.Cut(l, "\t")
		wantSet[id] = rest
	}
	gotSet := make(map[string]string, len(got))
	for _, l := range got {
		id, rest, _ := strings.Cut(l, "\t")
		gotSet[id] = rest
	}

	for id, w := range wantSet {
		g, ok := gotSet[id]
		if !ok {
			t.Errorf("check %q disappeared from the registry", id)
			continue
		}
		if g != w {
			t.Errorf("check %q changed:\n  was: %s\n  now: %s", id, w, g)
		}
	}
	for id := range gotSet {
		if _, ok := wantSet[id]; !ok {
			t.Errorf("check %q is new; regenerate the golden file if intended", id)
		}
	}
	if len(got) != len(want) {
		t.Errorf("check count changed: %d -> %d", len(want), len(got))
	}
}

// TestChecksIgnoreMalformedInput asserts that no check reports a finding
// against input it could not parse.
//
// A check that returns findings from a failed decode blames the manifest
// for the parser's confusion, and because the family is non-exemptable
// there is no way to suppress it. Rendering upstream of this can emit
// documents that are empty or not YAML at all, so this is reachable.
//
// Individual packages sampled this against their own checks; driving it
// from the registry covers all of them and any added later.
func TestChecksIgnoreMalformedInput(t *testing.T) {
	inputs := map[string]string{
		"unparseable": "not valid yaml {{",
		"empty":       "",
		"onlyComment": "# nothing here\n",
		"scalar":      "just-a-string\n",
		"list":        "- a\n- b\n",
		"nullKind":    "apiVersion: v1\nkind: ~\nmetadata:\n  name: test\n",
	}
	for _, c := range check.All() {
		if c.Section() != "runtime-validation" {
			continue
		}
		dc, ok := c.(interface {
			CheckDoc(data []byte, source string) []check.Finding
		})
		if !ok {
			t.Fatalf("check %q is not a DocCheck", c.ID())
		}
		t.Run(c.ID(), func(t *testing.T) {
			for name, in := range inputs {
				if got := dc.CheckDoc([]byte(in), "test.yaml"); len(got) != 0 {
					t.Errorf("%s: reported %d finding(s) on input it cannot parse: %v", name, len(got), got)
				}
			}
		})
	}
}

// TestChecksIgnoreKindsTheyDoNotDeclare asserts the applies-to contract for
// every registered check at once: a check handed a kind outside its list
// must be skipped by the dispatcher and, if run anyway, must report nothing.
//
// Driving it from the registry covers every check including ones added
// later, which per-package samples of the same property cannot.
func TestChecksIgnoreKindsTheyDoNotDeclare(t *testing.T) {
	// A syntactically valid document of a kind no runtime check declares.
	foreign := "apiVersion: example.com/v1\nkind: " + controlKind + "\nmetadata:\n  name: test\n"

	for _, c := range check.All() {
		if c.Section() != "runtime-validation" {
			continue
		}
		t.Run(c.ID(), func(t *testing.T) {
			sd, ok := c.(interface{ SkipDoc(string) bool })
			if !ok {
				t.Fatalf("check %q does not implement SkipDoc, so the dispatcher hands it every document", c.ID())
			}
			// A check declaring no kinds applies to all of them (object
			// metadata rules), so it is expected not to skip. Its Run must
			// still find nothing in an unrelated document.
			if len(appliesTo(c)) > 0 && !sd.SkipDoc(controlKind) {
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
