package kubernetes

import (
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
)

// TestRuntimeFindingsCarryResourceIdentity asserts that a finding names the
// object it is about, not just its kind.
//
// Two things depend on this. The report's Resource column is the only place a
// reader learns which object to fix, and dedupFindingsForTable keys on the
// identity: two different Jobs with the same violation and no Name collapse
// into one row, so fixing the reported one leaves a second, invisible failure.
//
// The fixtures reuse the defaulting corpus with a deliberate violation
// appended, so every kind already covered there is covered here too.
func TestRuntimeFindingsCarryResourceIdentity(t *testing.T) {
	manifests := map[string]string{
		"CronJob": `apiVersion: batch/v1
kind: CronJob
metadata:
  name: named-cronjob
spec:
  schedule: "not a schedule"
  jobTemplate:
    spec:
      template:
        spec:
          restartPolicy: Never
          containers:
            - name: c
              image: nginx:1.25
`,
		"Job": `apiVersion: batch/v1
kind: Job
metadata:
  name: named-job
spec:
  parallelism: -1
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: c
          image: nginx:1.25
`,
		"PodDisruptionBudget": `apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: named-pdb
spec:
  minAvailable: -1
  selector:
    matchLabels:
      app: x
`,
	}

	for kind, manifest := range manifests {
		t.Run(kind, func(t *testing.T) {
			var total int
			for _, c := range check.All() {
				if c.Section() != "runtime-validation" {
					continue
				}
				dc, ok := c.(interface {
					CheckDoc(data []byte, source string) []check.Finding
				})
				if !ok {
					continue
				}
				for _, f := range dc.CheckDoc([]byte(manifest), "id.yaml") {
					total++
					if f.Name == "" {
						t.Errorf("%s: finding has no resource name (Kind=%q, Message=%q)",
							f.CheckID, f.Kind, f.Message)
					}
				}
			}
			if total == 0 {
				t.Fatalf("fixture produced no findings, so it asserts nothing")
			}
		})
	}
}
