package admissionregistration

import (
	"strings"
	"testing"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// The admissionregistration rules had no behavioral coverage: the only test
// naming them proved one ID was registered.
//
// These six checks are three rules shared by two kinds through
// webhookCheckBase, so each case is run against both configurations. That is
// the part worth testing here - a rule that silently only worked for the
// mutating variant would look entirely healthy in a per-kind test.

type webhookCase struct {
	name     string
	webhooks string
	want     int
	contains string
}

// bothKinds runs each case against Mutating and Validating configurations,
// selecting the matching check from the pair the base constructor produces.
func runWebhookCases(t *testing.T, pick func([]runtime.Check) runtime.Check, cases []webhookCase) {
	t.Helper()
	kinds := []struct {
		kind  string
		base  webhookCheckBase
		field string
	}{
		{"MutatingWebhookConfiguration", mutatingWebhookBase, "mutating"},
		{"ValidatingWebhookConfiguration", validatingWebhookBase, "validating"},
	}
	for _, k := range kinds {
		c := pick(webhookChecks(k.base))
		for _, tc := range cases {
			t.Run(k.kind+"/"+tc.name, func(t *testing.T) {
				doc := "apiVersion: admissionregistration.k8s.io/v1\nkind: " + k.kind +
					"\nmetadata:\n  name: w\nwebhooks:\n" + tc.webhooks
				findings := c.Run([]byte(doc), "test.yaml")
				if len(findings) != tc.want {
					t.Fatalf("got %d finding(s), want %d: %v", len(findings), tc.want, findings)
				}
				if tc.contains != "" && !strings.Contains(findings[0].Message, tc.contains) {
					t.Errorf("message %q does not contain %q", findings[0].Message, tc.contains)
				}
			})
		}
	}
}

// hook wraps a clientConfig/webhook body as a single named webhook entry.
func hook(body string) string {
	return "  - name: w.example.com\n    admissionReviewVersions: [\"v1\"]\n    sideEffects: None\n" + body
}

func TestWebhookServiceInvalid(t *testing.T) {
	runWebhookCases(t, func(cs []runtime.Check) runtime.Check { return cs[0] }, []webhookCase{
		{
			name:     "service with a name",
			webhooks: hook("    clientConfig:\n      service:\n        name: svc\n        namespace: default\n"),
			want:     0,
		},
		{
			// A URL-based webhook has no service to validate.
			name:     "url instead of a service",
			webhooks: hook("    clientConfig:\n      url: https://example.com/hook\n"),
			want:     0,
		},
		{
			name:     "service with an empty name",
			webhooks: hook("    clientConfig:\n      service:\n        name: \"\"\n        namespace: default\n"),
			want:     1,
			contains: "must not be empty",
		},
	})
}

func TestWebhookFailurePolicyInvalid(t *testing.T) {
	runWebhookCases(t, func(cs []runtime.Check) runtime.Check { return cs[1] }, []webhookCase{
		{name: "Fail", webhooks: hook("    failurePolicy: Fail\n"), want: 0},
		{name: "Ignore", webhooks: hook("    failurePolicy: Ignore\n"), want: 0},
		// Defaulting supplies Fail, so an omitted value is not an error.
		{name: "omitted", webhooks: hook(""), want: 0},
		{name: "unknown", webhooks: hook("    failurePolicy: Retry\n"), want: 1, contains: "Retry"},
		{name: "lowercase is not the enum value", webhooks: hook("    failurePolicy: fail\n"), want: 1},
	})
}

func TestWebhookTimeoutInvalid(t *testing.T) {
	runWebhookCases(t, func(cs []runtime.Check) runtime.Check { return cs[2] }, []webhookCase{
		{name: "lower bound", webhooks: hook("    timeoutSeconds: 1\n"), want: 0},
		{name: "upper bound", webhooks: hook("    timeoutSeconds: 30\n"), want: 0},
		{name: "omitted", webhooks: hook(""), want: 0},
		{name: "zero", webhooks: hook("    timeoutSeconds: 0\n"), want: 1, contains: "between 1 and 30"},
		{name: "negative", webhooks: hook("    timeoutSeconds: -1\n"), want: 1},
		{name: "above the upper bound", webhooks: hook("    timeoutSeconds: 31\n"), want: 1},
	})
}

// TestWebhookChecksIgnoreTheOtherConfigurationKind pins the parse guard that
// keeps the two variants apart: each check decodes one concrete API type, and
// a mutating rule reporting on a validating document would attribute the
// finding to the wrong kind.
func TestWebhookChecksIgnoreTheOtherConfigurationKind(t *testing.T) {
	doc := "apiVersion: admissionregistration.k8s.io/v1\nkind: ValidatingWebhookConfiguration\n" +
		"metadata:\n  name: w\nwebhooks:\n" + hook("    timeoutSeconds: 999\n")

	for _, c := range webhookChecks(mutatingWebhookBase) {
		if got := c.Run([]byte(doc), "test.yaml"); len(got) != 0 {
			t.Errorf("%s reported %d finding(s) on a ValidatingWebhookConfiguration: %v", c.ID(), len(got), got)
		}
	}
}
