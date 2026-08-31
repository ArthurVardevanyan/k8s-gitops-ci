package main

import (
	"slices"
	"testing"
)

// refFile is a ref map shaped like a real upstream_refs.go: two shared digest
// constants, several entries using each, and a nested Additional ref carrying
// its own digest.
const refFile = "" +
	"var refs = map[string]runtime.UpstreamRef{\n" +
	"\t\"admissionregistration/service-invalid\": {\n" +
	"\t\tPath:        p,\n" +
	"\t\tDigest:      mutatingWebhookDigest,\n" +
	"\t\tValidatedAt: validatedAt,\n" +
	"\t},\n" +
	"\t\"admissionregistration/failure-policy-invalid\": {\n" +
	"\t\tPath:        p,\n" +
	"\t\tDigest:      mutatingWebhookDigest,\n" +
	"\t\tValidatedAt: validatedAt,\n" +
	"\t\tAdditional: []runtime.UpstreamRef{{\n" +
	"\t\t\tPath:        p,\n" +
	"\t\t\tDigest:      validatingWebhookDigest,\n" +
	"\t\t}},\n" +
	"\t},\n" +
	"\t\"admissionregistration/validating-service-invalid\": {\n" +
	"\t\tPath:        p,\n" +
	"\t\tDigest:      validatingWebhookDigest,\n" +
	"\t\tValidatedAt: validatedAt,\n" +
	"\t},\n" +
	"\t\"admissionregistration/validating-timeout-invalid\": {\n" +
	"\t\tPath:        p,\n" +
	"\t\tDigest:      validatingWebhookDigest,\n" +
	"\t\tValidatedAt: validatedAt,\n" +
	"\t},\n" +
	"}\n"

func TestEntriesReferencing(t *testing.T) {
	tests := []struct {
		name  string
		field string
		ident string
		want  []string
	}{
		{
			// The regression. A lazy scan over the whole file matched from
			// the first entry to the first use of the constant, naming a
			// mutating check as a user of the validating digest and hiding
			// every real user behind that one match.
			name:  "a later constant does not capture earlier entries",
			field: "Digest",
			ident: "validatingWebhookDigest",
			want: []string{
				"admissionregistration/validating-service-invalid",
				"admissionregistration/validating-timeout-invalid",
			},
		},
		{
			name:  "every user of a shared constant is returned",
			field: "Digest",
			ident: "mutatingWebhookDigest",
			want: []string{
				"admissionregistration/service-invalid",
				"admissionregistration/failure-policy-invalid",
			},
		},
		{
			name:  "an unused identifier has no users",
			field: "Digest",
			ident: "someOtherDigest",
			want:  []string{},
		},
		{
			name:  "a different field is not matched",
			field: "ValidatedAt",
			ident: "mutatingWebhookDigest",
			want:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := entriesReferencing(refFile, tt.field, tt.ident)
			if !slices.Equal(got, tt.want) {
				t.Errorf("entriesReferencing(%q, %q) = %v, want %v", tt.field, tt.ident, got, tt.want)
			}
		})
	}
}

// TestEntriesReferencingIgnoresNestedRefs pins that a supporting citation's
// digest is not attributed to the entry that contains it. The entry for
// failure-policy-invalid nests an Additional ref using validatingWebhookDigest
// while its own digest is the mutating one; counting the nested value would
// make --update compare the primary digest against a supporting citation's.
func TestEntriesReferencingIgnoresNestedRefs(t *testing.T) {
	got := entriesReferencing(refFile, "Digest", "validatingWebhookDigest")
	if slices.Contains(got, "admissionregistration/failure-policy-invalid") {
		t.Errorf("nested Additional digest attributed to the entry: %v", got)
	}
}
