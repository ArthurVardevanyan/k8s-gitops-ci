package crb

import "testing"

func TestValidateBytes_MissingNamespace(t *testing.T) {
	data := []byte(`kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: binding
subjects:
- kind: ServiceAccount
  name: sa
roleRef:
  kind: ClusterRole
  name: role
`)
	errs := ValidateBytes(data, "crb.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error: %v", errs)
	}
	want := "crb.yaml: ClusterRoleBinding \"binding\" subject sa: ServiceAccount subject missing namespace (will default to 'default')"
	if errs[0].String() != want {
		t.Errorf("got %q want %q", errs[0].String(), want)
	}
}

func TestValidateBytes_WithNamespace(t *testing.T) {
	data := []byte(`kind: ClusterRoleBinding
metadata:
  name: binding
subjects:
- kind: ServiceAccount
  name: sa
  namespace: ns
`)
	errs := ValidateBytes(data, "crb.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors: %v", errs)
	}
}

func TestValidateBytes_NonServiceAccountSubject(t *testing.T) {
	data := []byte(`kind: ClusterRoleBinding
metadata:
  name: binding
subjects:
- kind: User
  name: user
`)
	errs := ValidateBytes(data, "crb.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors for User subject: %v", errs)
	}
}

func TestDeduplicate(t *testing.T) {
	errs := []ValidationError{
		{Kind: "ClusterRoleBinding", Name: "b", Subject: "sa"},
		{Kind: "ClusterRoleBinding", Name: "b", Subject: "sa"},
	}
	ded := Deduplicate(errs)
	if len(ded) != 1 || ded[0].Count != 2 {
		t.Errorf("unexpected dedup: %v", ded)
	}
}
