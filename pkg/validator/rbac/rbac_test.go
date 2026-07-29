package rbac

import (
	"strings"
	"testing"
)

func TestValidateReader_ReadonlyAggregateWithBadVerbs(t *testing.T) {
	data := `kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: reader
  labels:
    rbac.authorization.k8s.io/aggregate-to-view: "true"
rules:
- verbs: ["get", "create"]
  resources: ["pods"]
  apiGroups: [""]
`
	errs := ValidateReader(strings.NewReader(data), "cr.yaml")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error: %v", errs)
	}
	if !strings.Contains(errs[0].String(), "non-readonly verbs") {
		t.Errorf("unexpected error: %q", errs[0].String())
	}
}

func TestValidateReader_NoAggregateLabel(t *testing.T) {
	data := `kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: cr
rules:
- verbs: ["create"]
  resources: ["pods"]
`
	errs := ValidateReader(strings.NewReader(data), "cr.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors without aggregate label: %v", errs)
	}
}

func TestValidateWildcards_VerbWildcard(t *testing.T) {
	data := `kind: ClusterRole
metadata:
  name: admin
rules:
- verbs: ["*"]
  resources: ["pods"]
`
	errs := ValidateWildcardsReader(strings.NewReader(data), "cr.yaml")
	if len(errs) != 1 || errs[0].Field != "verbs" {
		t.Errorf("expected verb wildcard: %v", errs)
	}
}

func TestValidateWildcards_ResourceWildcard(t *testing.T) {
	data := `kind: Role
metadata:
  name: admin
rules:
- verbs: ["get"]
  resources: ["*"]
`
	errs := ValidateWildcardsReader(strings.NewReader(data), "role.yaml")
	if len(errs) != 1 || errs[0].Field != "resources" {
		t.Errorf("expected resource wildcard: %v", errs)
	}
}

func TestFormatWildcardComment(t *testing.T) {
	s := FormatWildcardComment([]WildcardError{{Kind: "ClusterRole", Resource: "admin", RuleIndex: 0, Field: "verbs"}})
	if !strings.Contains(s, WildcardMarker) {
		t.Errorf("expected marker: %q", s)
	}
}
