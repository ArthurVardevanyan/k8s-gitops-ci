package validation

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// nestedString returns the string at path in obj, or "" if it is absent or is
// not a string.
func nestedString(obj map[string]interface{}, path ...string) string {
	value, found, err := unstructured.NestedString(obj, path...)
	if err != nil || !found {
		return ""
	}

	return value
}

// parseKind decodes a manifest and returns it along with its metadata.name,
// but only if it is of the requested kind. A nil map means "not this kind, or
// not parseable", which every caller treats as nothing to validate.
//
// The kind test is redundant with the adapter's Kinds() gate during a normal
// pipeline run, but it is kept so that a check invoked directly, as unit tests
// do, still only reports on the kind it was written for.
func parseKind(data []byte, kind string) (obj map[string]interface{}, name string) {
	if err := yaml.Unmarshal(data, &obj); err != nil {
		return nil, ""
	}

	if nestedString(obj, "kind") != kind {
		return nil, ""
	}

	return obj, nestedString(obj, "metadata", "name")
}

// enumFieldFindings reports a finding when the string at path in obj is set to
// something outside allowed, mirroring the Unsupported value message the API
// server produces for a closed enum field.
//
// label is the field name used in the message, which upstream reports relative
// to the enclosing struct rather than as a full path. When allowEmpty is set,
// an empty value is accepted because the API server defaults it.
func enumFieldFindings(
	c runtime.Check,
	obj map[string]interface{},
	kind, name, label string,
	path []string,
	allowed []string,
	allowEmpty bool,
) []runtime.Finding {
	if obj == nil || len(path) == 0 {
		return nil
	}

	value, found, err := unstructured.NestedString(obj, path...)
	if err != nil || !found {
		return nil
	}

	if allowEmpty && value == "" {
		return nil
	}

	if slices.Contains(allowed, value) {
		return nil
	}

	quoted := make([]string, 0, len(allowed))
	for _, a := range allowed {
		quoted = append(quoted, strconv.Quote(a))
	}

	fieldPath := field.NewPath(path[0])
	for _, p := range path[1:] {
		fieldPath = fieldPath.Child(p)
	}

	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Category:  c.Category(),
		Finding: check.Finding{
			Path:    fieldPath.String(),
			Message: fmt.Sprintf("%s: Unsupported value: %q: supported values: %s", label, value, strings.Join(quoted, ", ")),
			Kind:    kind,
			Name:    name,
			Value:   value,
		},
	}}
}
