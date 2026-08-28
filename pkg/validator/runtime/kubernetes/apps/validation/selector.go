package validation

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// selectorInvalidFindings verifies spec.selector is a valid label selector for
// a workload object decoded into obj, reporting findings attributed to the
// given check, kind and object name. Deployment, DaemonSet and ReplicaSet all
// share this validation.
// Source: k8s.io/kubernetes/pkg/apis/apps/validation/validation.go
func selectorInvalidFindings(c runtime.Check, obj map[string]interface{}, kind, name string) []runtime.Finding {
	if obj == nil {
		return nil
	}

	selectorMap, found, _ := unstructured.NestedMap(obj, "spec", "selector")
	if !found {
		return nil
	}

	finding := func(path, key string, errs []string) []runtime.Finding {
		return []runtime.Finding{{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    path,
				Message: fmt.Sprintf("invalid label selector key %q: %s", key, strings.Join(errs, ", ")),
				Kind:    kind,
				Name:    name,
			},
		}}
	}

	// Check matchLabels keys (only string values in the selector map)
	for key, val := range selectorMap {
		if _, ok := val.(string); !ok {
			continue
		}
		if errs := validation.IsQualifiedName(key); len(errs) > 0 {
			return finding(
				field.NewPath("spec").Child("selector").Child("matchLabels").Key(key).String(),
				key, errs,
			)
		}
	}

	// Check matchExpressions keys
	matchExpressionsList, found, err := unstructured.NestedSlice(obj, "spec", "selector", "matchExpressions")
	if err != nil || !found {
		return nil
	}

	for _, rawExpr := range matchExpressionsList {
		exprMap, ok := rawExpr.(map[string]interface{})
		if !ok {
			continue
		}
		key, _ := exprMap["key"].(string)
		if key == "" {
			continue
		}
		if errs := validation.IsQualifiedName(key); len(errs) > 0 {
			return finding(
				field.NewPath("spec").Child("selector").Child("matchExpressions").Child("key").String(),
				key, errs,
			)
		}
	}

	return nil
}
