package policy

import (
	"strings"
	"testing"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// docCase is one complete manifest and the findings it must produce. These
// rules span three kinds with unrelated spec shapes, so a case carries the
// whole document rather than a fragment.
type docCase struct {
	name     string
	doc      string
	want     int
	contains string
}

func runDocCases(t *testing.T, run func([]byte, string) []runtime.Finding, cases []docCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := run([]byte(tc.doc), "test.yaml")
			if len(findings) != tc.want {
				t.Fatalf("got %d finding(s), want %d: %v", len(findings), tc.want, findings)
			}
			if tc.contains != "" && !strings.Contains(findings[0].Message, tc.contains) {
				t.Errorf("message %q does not contain %q", findings[0].Message, tc.contains)
			}
		})
	}
}

func TestSelectorInvalid(t *testing.T) {
	runDocCases(t, newSelectorInvalidCheck().Run, []docCase{
		{name: "PDBSelectorValid", doc: "kind: \"PodDisruptionBudget\"\nmetadata:\n  name: \"test\"\nspec:\n  selector:\n    matchLabels:\n      app: \"test\"\n  minAvailable: 1\n", want: 0},
	})
}

func TestMinAvailableInvalid(t *testing.T) {
	runDocCases(t, newMinAvailableInvalidCheck().Run, []docCase{
		{name: "PDBMinAvailableCheck", doc: "kind: \"PodDisruptionBudget\"\nmetadata:\n  name: \"test\"\nspec:\n  selector:\n    matchLabels:\n      app: \"test\"\n  minAvailable: -1\n", want: 1},
		{name: "PDBMinAvailableValid", doc: "kind: \"PodDisruptionBudget\"\nmetadata:\n  name: \"test\"\nspec:\n  selector:\n    matchLabels:\n      app: \"test\"\n  minAvailable: 1\n", want: 0},
	})
}

func TestMaxUnavailableInvalid(t *testing.T) {
	runDocCases(t, newMaxUnavailableInvalidCheck().Run, []docCase{
		{name: "PDBMaxUnavailableCheck", doc: "kind: \"PodDisruptionBudget\"\nmetadata:\n  name: \"test\"\nspec:\n  selector:\n    matchLabels:\n      app: \"test\"\n  maxUnavailable: -1\n", want: 1},
		{name: "PDBMaxUnavailableValid", doc: "kind: \"PodDisruptionBudget\"\nmetadata:\n  name: \"test\"\nspec:\n  selector:\n    matchLabels:\n      app: \"test\"\n  maxUnavailable: 1\n", want: 0},
	})
}

func TestMinAndMaxSpecified(t *testing.T) {
	runDocCases(t, newMinAndMaxSpecifiedCheck().Run, []docCase{
		{name: "PDBMinAndMaxSpecifiedCheck", doc: "kind: \"PodDisruptionBudget\"\nmetadata:\n  name: \"test\"\nspec:\n  selector:\n    matchLabels:\n      app: \"test\"\n  minAvailable: 1\n  maxUnavailable: 1\n", want: 1},
		{name: "PDBMinAndMaxSpecifiedValid", doc: "kind: \"PodDisruptionBudget\"\nmetadata:\n  name: \"test\"\nspec:\n  selector:\n    matchLabels:\n      app: \"test\"\n  minAvailable: 1\n", want: 0},
	})
}

// The selector must be validated as a structured LabelSelector. It was
// previously flattened to a string and parsed with labels.Parse, which has
// no representation for matchExpressions - so every operator/values rule
// was skipped. The old fixture passed a bare string selector, which is not
// a valid PodDisruptionBudget shape at all.
func TestPDBSelectorCheck(t *testing.T) {
	tests := []struct {
		name string
		spec string
	}{
		{
			"invalid matchLabels key",
			`  selector:
    matchLabels:
      "invalid key with spaces": myapp
  minAvailable: 1`,
		},
		{
			// Unreachable through a stringified selector: this is the case
			// the previous implementation silently accepted.
			"invalid matchExpressions key",
			`  selector:
    matchExpressions:
    - key: "invalid key with spaces"
      operator: In
      values:
      - val
  minAvailable: 1`,
		},
		{
			"In operator with no values",
			`  selector:
    matchExpressions:
    - key: app
      operator: In
      values: []
  minAvailable: 1`,
		},
		{
			"unknown operator",
			`  selector:
    matchExpressions:
    - key: app
      operator: Sometimes
      values:
      - val
  minAvailable: 1`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte("kind: PodDisruptionBudget\nmetadata:\n  name: test\nspec:\n" + tt.spec + "\n")
			findings := newSelectorInvalidCheck().Run(data, "test.yaml")
			if len(findings) == 0 {
				t.Fatalf("expected at least 1 finding, got none")
			}
			if findings[0].RuleID != "kubernetes/policy/selector-invalid" {
				t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
			}
			if findings[0].Kind != "PodDisruptionBudget" {
				t.Errorf("unexpected kind: %s", findings[0].Kind)
			}
		})
	}
}
