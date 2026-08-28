package validation

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// replicaSetSelectorInvalidCheck verifies selector is a valid label selector.
// Source: k8s.io/kubernetes/pkg/apis/apps/validation/validation.go
type replicaSetSelectorInvalidCheck struct{}

func (c replicaSetSelectorInvalidCheck) ID() string {
	return "apps/replicaset-selector-invalid"
}

func (c replicaSetSelectorInvalidCheck) Title() string {
	return "Selector Must Be A Valid Label Selector"
}

func (c replicaSetSelectorInvalidCheck) Category() string {
	return "apps"
}

func (c replicaSetSelectorInvalidCheck) Blocking() bool {
	return true
}

func (c replicaSetSelectorInvalidCheck) RenderSensitive() bool {
	return true
}

func (c replicaSetSelectorInvalidCheck) Kinds() []string {
	return []string{"ReplicaSet"}
}

func (c replicaSetSelectorInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	obj, name := parseKind(data, "ReplicaSet")

	return selectorInvalidFindings(c, obj, "ReplicaSet", name)
}

// replicaSetReplicasInvalidCheck verifies replicas >= 0.
// Source: k8s.io/kubernetes/pkg/apis/apps/validation/validation.go
type replicaSetReplicasInvalidCheck struct{}

func (c replicaSetReplicasInvalidCheck) ID() string {
	return "apps/replicaset-replicas-invalid"
}

func (c replicaSetReplicasInvalidCheck) Title() string {
	return "Replicas Must Be >= 0"
}

func (c replicaSetReplicasInvalidCheck) Category() string {
	return "apps"
}

func (c replicaSetReplicasInvalidCheck) Blocking() bool {
	return true
}

func (c replicaSetReplicasInvalidCheck) RenderSensitive() bool {
	return true
}

func (c replicaSetReplicasInvalidCheck) Kinds() []string {
	return []string{"ReplicaSet"}
}

func (c replicaSetReplicasInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	specMap, name := parseKind(data, "ReplicaSet")
	if specMap == nil {
		return nil
	}

	replicas, found, err := unstructured.NestedInt64(specMap, "spec", "replicas")
	if err != nil || !found {
		return nil
	}

	if replicas >= 0 {
		return nil
	}

	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Category:  c.Category(),
		Finding: check.Finding{
			Path:    field.NewPath("spec").Child("replicas").String(),
			Message: fmt.Sprintf("replicas: must be >= 0, got %d", replicas),
			Kind:    "ReplicaSet",
			Name:    name,
			Value:   fmt.Sprintf("%d", replicas),
		},
	}}
}

// ValidateReplicaSet runs all replicaset validation checks and returns findings.
func ValidateReplicaSet(data []byte, source string) []runtime.Finding {
	checks := []runtime.Check{
		replicaSetSelectorInvalidCheck{},
		replicaSetReplicasInvalidCheck{},
	}
	findings := make([]runtime.Finding, 0, len(checks))
	for _, c := range checks {
		findings = append(findings, c.Run(data, source)...)
	}
	return findings
}

// init registers all replicaset checks.
func init() {
	Register()
}
