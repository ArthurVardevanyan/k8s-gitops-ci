package validation

import (
	"fmt"
	"sort"
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

	if _, found, _ := unstructured.NestedMap(obj, "spec", "selector"); !found {
		return nil
	}

	finding := func(path, key string, errs []string) []runtime.Finding {
		return []runtime.Finding{{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Finding: check.Finding{
				Path:    path,
				Message: fmt.Sprintf("invalid label selector key %q: %s", key, strings.Join(errs, ", ")),
				Kind:    kind,
				Name:    name,
			},
		}}
	}

	// Validate matchLabels keys. This must read spec.selector.matchLabels,
	// not spec.selector, whose own values are a map and a slice rather than
	// label keys.
	matchLabels, found, err := unstructured.NestedMap(obj, "spec", "selector", "matchLabels")
	if err == nil && found {
		// Sorted so that a selector with more than one invalid key reports
		// the same one on every run; map iteration order would otherwise
		// make the message nondeterministic.
		keys := make([]string, 0, len(matchLabels))
		for key := range matchLabels {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if errs := validation.IsQualifiedName(key); len(errs) > 0 {
				return finding(
					field.NewPath("spec").Child("selector").Child("matchLabels").Key(key).String(),
					key, errs,
				)
			}
		}
	}

	// Check matchExpressions keys
	matchExpressionsList, found, err := unstructured.NestedSlice(obj, "spec", "selector", "matchExpressions")
	if err != nil || !found {
		return nil
	}

	for i, rawExpr := range matchExpressionsList {
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
				// Indexed: spec.selector.matchExpressions.key is not a path
				// any manifest has, so it cannot be located or fixed.
				field.NewPath("spec").Child("selector").Child("matchExpressions").Index(i).Child("key").String(),
				key, errs,
			)
		}
	}

	return nil
}
