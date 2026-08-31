package core

import (
	"strings"
	"testing"
)

func objectMetaDoc(kind, name, generateName, namespace string) []byte {
	var b strings.Builder
	b.WriteString("apiVersion: v1\nkind: " + kind + "\nmetadata:\n")
	if name != "" {
		b.WriteString("  name: \"" + name + "\"\n")
	}
	if generateName != "" {
		b.WriteString("  generateName: \"" + generateName + "\"\n")
	}
	if namespace != "" {
		b.WriteString("  namespace: \"" + namespace + "\"\n")
	}
	if name == "" && generateName == "" && namespace == "" {
		b.WriteString("  labels: {}\n")
	}
	return []byte(b.String())
}

func TestObjectMetaNameInvalidCheck(t *testing.T) {
	tests := []struct {
		name        string
		kind        string
		objectName  string
		wantFinding bool
		wantMessage string
	}{
		// DNS-1123 subdomain kinds (max 253, lowercase alphanumeric, '-', '.').
		{name: "deployment valid", kind: "Deployment", objectName: "my-app", wantFinding: false},
		{name: "deployment dots allowed in subdomain", kind: "Deployment", objectName: "my.app", wantFinding: false},
		{name: "deployment leading digit allowed", kind: "Deployment", objectName: "1st-app", wantFinding: false},
		{name: "deployment uppercase rejected", kind: "Deployment", objectName: "MyApp", wantFinding: true},
		{name: "deployment underscore rejected", kind: "Deployment", objectName: "my_app", wantFinding: true},
		{name: "deployment trailing hyphen rejected", kind: "Deployment", objectName: "my-app-", wantFinding: true},
		{name: "deployment 253 chars allowed", kind: "Deployment", objectName: strings.Repeat("a", 253), wantFinding: false},
		{name: "deployment 254 chars rejected", kind: "Deployment", objectName: strings.Repeat("a", 254), wantFinding: true},

		// StatefulSet uses DNS-1123 *label*, not subdomain: max 63, no dots.
		// Upstream apps.ValidateStatefulSetName -> NameIsDNSLabel.
		{name: "statefulset valid", kind: "StatefulSet", objectName: "my-db", wantFinding: false},
		{name: "statefulset dots rejected (label not subdomain)", kind: "StatefulSet", objectName: "my.db", wantFinding: true},
		{name: "statefulset 63 chars allowed", kind: "StatefulSet", objectName: strings.Repeat("a", 63), wantFinding: false},
		{name: "statefulset 64 chars rejected", kind: "StatefulSet", objectName: strings.Repeat("a", 64), wantFinding: true},

		// Namespace uses DNS-1123 label as well.
		{name: "namespace valid", kind: "Namespace", objectName: "team-a", wantFinding: false},
		{name: "namespace dots rejected", kind: "Namespace", objectName: "team.a", wantFinding: true},

		// Service: KEP-5311 relaxed rule (DNS-1123 label). A leading digit
		// must NOT be flagged - on a cluster with the gate enabled (default
		// since 1.36, locked on in 1.37) such a name is perfectly valid, and
		// these findings are blocking and non-exemptable.
		{name: "service leading digit not flagged (relaxed rule)", kind: "Service", objectName: "1st-api", wantFinding: false},
		{name: "service valid", kind: "Service", objectName: "api", wantFinding: false},
		{name: "service uppercase rejected", kind: "Service", objectName: "API", wantFinding: true},
		{name: "service dots rejected", kind: "Service", objectName: "api.v2", wantFinding: true},

		// RBAC uses IsPathSegmentName, which is far laxer than DNS. These
		// are the regression guards: flagging them would block valid RBAC.
		{name: "role with dots is valid", kind: "Role", objectName: "My.Role", wantFinding: false},
		{name: "clusterrole uppercase is valid", kind: "ClusterRole", objectName: "SystemAdmin", wantFinding: false},
		{name: "clusterrole colon is valid", kind: "ClusterRole", objectName: "system:controller:foo", wantFinding: false},
		{name: "rolebinding over 253 chars is valid", kind: "RoleBinding", objectName: strings.Repeat("a", 300), wantFinding: false},
		{name: "role with slash rejected", kind: "Role", objectName: "foo/bar", wantFinding: true},
		{name: "role with percent rejected", kind: "ClusterRoleBinding", objectName: "foo%bar", wantFinding: true},
		{name: "role named dot rejected", kind: "Role", objectName: ".", wantFinding: true},

		// Unknown kinds are skipped rather than defaulted, so a custom
		// resource with a non-DNS name is never falsely blocked.
		{name: "unknown kind skipped", kind: "SomeCustomResource", objectName: "Not_A_DNS_Name", wantFinding: false},
		{name: "CRD deliberately not covered here", kind: "CustomResourceDefinition", objectName: "BadName", wantFinding: false},
	}

	c := newObjectMetaNameInvalidCheck()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.Run(objectMetaDoc(tt.kind, tt.objectName, "", ""), "test.yaml")
			if tt.wantFinding && len(got) == 0 {
				t.Fatalf("expected a finding for %s/%q, got none", tt.kind, tt.objectName)
			}
			if !tt.wantFinding && len(got) != 0 {
				t.Fatalf("expected no finding for %s/%q, got: %s", tt.kind, tt.objectName, got[0].Message)
			}
		})
	}
}

// TestObjectMetaNameGenerateName pins upstream's rule that a name is required
// only when generateName is also absent (ValidateObjectMetaAccessor). An
// object supplying only generateName is valid, and the previous per-kind
// checks got this wrong.
func TestObjectMetaNameGenerateName(t *testing.T) {
	c := newObjectMetaNameInvalidCheck()

	t.Run("generateName only is valid", func(t *testing.T) {
		if got := c.Run(objectMetaDoc("ConfigMap", "", "my-prefix-", ""), "t.yaml"); len(got) != 0 {
			t.Errorf("generateName-only object must be valid, got: %s", got[0].Message)
		}
	})

	t.Run("neither name nor generateName is an error", func(t *testing.T) {
		got := c.Run(objectMetaDoc("ConfigMap", "", "", ""), "t.yaml")
		if len(got) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(got))
		}
		if !strings.Contains(got[0].Message, "name or generateName is required") {
			t.Errorf("unexpected message: %s", got[0].Message)
		}
	})

	t.Run("invalid generateName is validated too", func(t *testing.T) {
		got := c.Run(objectMetaDoc("ConfigMap", "", "Bad_Prefix", ""), "t.yaml")
		if len(got) == 0 {
			t.Fatal("expected a finding for an invalid generateName")
		}
		if got[0].Path != "metadata.generateName" {
			t.Errorf("expected path metadata.generateName, got %s", got[0].Path)
		}
	})
}

func TestObjectMetaNamespaceInvalidCheck(t *testing.T) {
	tests := []struct {
		name        string
		namespace   string
		wantFinding bool
	}{
		{name: "valid namespace", namespace: "team-a", wantFinding: false},
		{name: "absent namespace is not checked", namespace: "", wantFinding: false},
		{name: "dots rejected (label not subdomain)", namespace: "team.a", wantFinding: true},
		{name: "uppercase rejected", namespace: "TeamA", wantFinding: true},
		{name: "underscore rejected", namespace: "team_a", wantFinding: true},
		{name: "63 chars allowed", namespace: strings.Repeat("a", 63), wantFinding: false},
		{name: "64 chars rejected", namespace: strings.Repeat("a", 64), wantFinding: true},
	}

	c := newObjectMetaNamespaceInvalidCheck()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.Run(objectMetaDoc("ConfigMap", "cm", "", tt.namespace), "test.yaml")
			if tt.wantFinding && len(got) == 0 {
				t.Fatalf("expected a finding for namespace %q", tt.namespace)
			}
			if !tt.wantFinding && len(got) != 0 {
				t.Fatalf("expected no finding for namespace %q, got: %s", tt.namespace, got[0].Message)
			}
		})
	}
}

// TestObjectMetaNamespaceScopeNotChecked documents the deliberate division of
// responsibility: presence and cluster-scope forbidden-ness of
// metadata.namespace belong to the exemptable "namespace" static check, which
// owns the generated resource-scope map. This check validates format only.
func TestObjectMetaNamespaceScopeNotChecked(t *testing.T) {
	c := newObjectMetaNamespaceInvalidCheck()

	if got := c.Run(objectMetaDoc("ConfigMap", "cm", "", ""), "t.yaml"); len(got) != 0 {
		t.Errorf("a missing namespace must not be reported here, got: %s", got[0].Message)
	}
	// A cluster-scoped object carrying a (well-formed) namespace is also not
	// this check's problem.
	if got := c.Run(objectMetaDoc("Namespace", "ns", "", "team-a"), "t.yaml"); len(got) != 0 {
		t.Errorf("cluster-scope namespace misuse must not be reported here, got: %s", got[0].Message)
	}
}
