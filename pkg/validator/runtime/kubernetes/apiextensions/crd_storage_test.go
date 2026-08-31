package apiextensions

import "testing"

// TestCRDStorageVersionCountsEmptyVersions covers the shape upstream rejects
// with storageFlagCount != 1 that a guard on spec.versions being non-empty
// used to skip: a CRD with no versions has no storage version either.
func TestCRDStorageVersionCountsEmptyVersions(t *testing.T) {
	const head = "apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\n" +
		"metadata:\n  name: widgets.example.com\nspec:\n  group: example.com\n"

	tests := []struct {
		name string
		spec string
		want int
	}{
		{"no versions field", "", 1},
		{"empty versions list", "  versions: []\n", 1},
		{"one storage version", "  versions:\n    - name: v1\n      storage: true\n      served: true\n", 0},
		{"no version marked storage", "  versions:\n    - name: v1\n      storage: false\n      served: true\n", 1},
		{
			"two storage versions",
			"  versions:\n    - name: v1\n      storage: true\n      served: true\n" +
				"    - name: v2\n      storage: true\n      served: true\n",
			1,
		},
	}

	c := newStorageVersionInvalidCheck()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := c.Run([]byte(head+tc.spec), "crd.yaml")
			if len(got) != tc.want {
				t.Fatalf("got %d finding(s), want %d: %v", len(got), tc.want, got)
			}
		})
	}
}
