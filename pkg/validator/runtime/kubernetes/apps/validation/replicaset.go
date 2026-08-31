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
type replicaSetSelectorInvalidCheck struct{ runtime.Meta }

func newReplicaSetSelectorInvalidCheck() replicaSetSelectorInvalidCheck {
	return replicaSetSelectorInvalidCheck{runtime.Meta{
		RuleID:    "apps/replicaset-selector-invalid",
		RuleTitle: "Selector Must Be A Valid Label Selector",
		AppliesTo: []string{"ReplicaSet"},
	}}
}

func (c replicaSetSelectorInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	obj, name := parseKind(data, "ReplicaSet")

	return selectorInvalidFindings(c, obj, "ReplicaSet", name)
}

// replicaSetReplicasInvalidCheck verifies replicas >= 0.
// Source: k8s.io/kubernetes/pkg/apis/apps/validation/validation.go
type replicaSetReplicasInvalidCheck struct{ runtime.Meta }

func newReplicaSetReplicasInvalidCheck() replicaSetReplicasInvalidCheck {
	return replicaSetReplicasInvalidCheck{runtime.Meta{
		RuleID:    "apps/replicaset-replicas-invalid",
		RuleTitle: "Replicas Must Be >= 0",
		AppliesTo: []string{"ReplicaSet"},
	}}
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
		Finding: check.Finding{
			Path:    field.NewPath("spec").Child("replicas").String(),
			Message: fmt.Sprintf("replicas: must be >= 0, got %d", replicas),
			Kind:    "ReplicaSet",
			Name:    name,
			Value:   fmt.Sprintf("%d", replicas),
		},
	}}
}

// init registers all replicaset checks.
func init() {
	Register()
}
