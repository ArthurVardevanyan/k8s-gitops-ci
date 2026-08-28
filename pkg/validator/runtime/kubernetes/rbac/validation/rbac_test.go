package validation

import (
	"strings"
	"testing"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// The RBAC rules had no behavioral coverage: the only runtime test naming
// them proved they were registered. These are also the rules most exposed by
// the dispatch defect fixed alongside them - a RoleBinding whose roleRef.kind
// preceded its own kind was dispatched as a ClusterRole, so every rule here
// was skipped on exactly the documents it exists to check.

type rbacCase struct {
	name     string
	doc      string
	want     int
	contains string
}

func runRBACCases(t *testing.T, run func([]byte, string) []runtime.Finding, cases []rbacCase) {
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

// binding builds a RoleBinding or ClusterRoleBinding with the given roleRef
// and subjects blocks.
func binding(kind, roleRef, subjects string) string {
	s := "apiVersion: rbac.authorization.k8s.io/v1\nkind: " + kind + "\nmetadata:\n  name: b\n"
	if roleRef != "" {
		s += "roleRef:\n" + roleRef
	}
	if subjects != "" {
		s += "subjects:\n" + subjects
	}
	return s
}

const goodRoleRef = "  apiGroup: rbac.authorization.k8s.io\n  kind: Role\n  name: r\n"

func TestRoleBindingRoleRefInvalid(t *testing.T) {
	runRBACCases(t, newRoleBindingRoleRefInvalidCheck().Run, []rbacCase{
		{name: "Role reference", doc: binding("RoleBinding", goodRoleRef, ""), want: 0},
		{
			name: "ClusterRole reference is also valid from a RoleBinding",
			doc:  binding("RoleBinding", "  apiGroup: rbac.authorization.k8s.io\n  kind: ClusterRole\n  name: r\n", ""),
			want: 0,
		},
		{
			name:     "unsupported roleRef kind",
			doc:      binding("RoleBinding", "  apiGroup: rbac.authorization.k8s.io\n  kind: ServiceAccount\n  name: r\n", ""),
			want:     1,
			contains: "must be Role or ClusterRole",
		},
		{
			name:     "missing roleRef name",
			doc:      binding("RoleBinding", "  apiGroup: rbac.authorization.k8s.io\n  kind: Role\n", ""),
			want:     1,
			contains: "name is required",
		},
		{
			name:     "wrong apiGroup",
			doc:      binding("RoleBinding", "  apiGroup: \"\"\n  kind: Role\n  name: r\n", ""),
			want:     1,
			contains: "does not match expected group",
		},
		{
			// An absent roleRef fails all three branches at once; the rule
			// reports each rather than stopping at the first.
			name: "absent roleRef reports every branch",
			doc:  binding("RoleBinding", "", ""),
			want: 3,
		},
		{
			name: "a ClusterRoleBinding is not this rule's kind",
			doc:  binding("ClusterRoleBinding", goodRoleRef, ""),
			want: 0,
		},
	})
}

func TestClusterRoleBindingRoleRefInvalid(t *testing.T) {
	runRBACCases(t, newClusterRoleBindingRoleRefInvalidCheck().Run, []rbacCase{
		{
			name: "ClusterRole reference",
			doc:  binding("ClusterRoleBinding", "  apiGroup: rbac.authorization.k8s.io\n  kind: ClusterRole\n  name: r\n", ""),
			want: 0,
		},
		{
			// A ClusterRoleBinding may only reference a ClusterRole; a
			// namespaced Role has no meaning cluster-wide.
			name:     "Role reference is not valid cluster-wide",
			doc:      binding("ClusterRoleBinding", goodRoleRef, ""),
			want:     1,
			contains: "ClusterRole",
		},
		{
			name:     "missing name",
			doc:      binding("ClusterRoleBinding", "  apiGroup: rbac.authorization.k8s.io\n  kind: ClusterRole\n", ""),
			want:     1,
			contains: "name is required",
		},
		{
			name: "a RoleBinding is not this rule's kind",
			doc:  binding("RoleBinding", goodRoleRef, ""),
			want: 0,
		},
	})
}

func TestClusterRoleBindingSubjectInvalid(t *testing.T) {
	crb := func(subjects string) string {
		return binding("ClusterRoleBinding",
			"  apiGroup: rbac.authorization.k8s.io\n  kind: ClusterRole\n  name: r\n", subjects)
	}
	runRBACCases(t, newClusterRoleBindingSubjectInvalidCheck().Run, []rbacCase{
		{name: "User", doc: crb("  - kind: User\n    name: alice\n"), want: 0},
		{name: "Group", doc: crb("  - kind: Group\n    name: devs\n"), want: 0},
		{
			name: "ServiceAccount",
			doc:  crb("  - kind: ServiceAccount\n    name: sa\n    namespace: default\n"),
			want: 0,
		},
		{name: "no subjects", doc: crb(""), want: 0},
		{
			name:     "unsupported subject kind",
			doc:      crb("  - kind: Robot\n    name: r2\n"),
			want:     1,
			contains: "must be User, Group, or ServiceAccount",
		},
		{
			name:     "missing subject name",
			doc:      crb("  - kind: User\n"),
			want:     1,
			contains: "name is required",
		},
		{
			name: "each invalid subject is reported",
			doc:  crb("  - kind: Robot\n    name: r2\n  - kind: User\n"),
			want: 2,
		},
	})
}
