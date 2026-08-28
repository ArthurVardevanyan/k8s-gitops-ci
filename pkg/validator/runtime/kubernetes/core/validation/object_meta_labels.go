package validation

import (
	"fmt"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// This file ports the label and annotation rules the API server applies to
// every object, from ValidateLabels
// (k8s.io/apimachinery/pkg/apis/meta/v1/validation/validation.go) and
// ValidateAnnotations (k8s.io/apimachinery/pkg/api/validation/objectmeta.go),
// both reached from ValidateObjectMeta.
//
// Unlike the name rules in object_meta.go these are not per-kind: every
// object the API server accepts goes through them, so these checks declare no
// Kinds() and run against every document.
//
// None of it is expressible in the embedded schemas, where metadata.labels
// and metadata.annotations are open string maps with no key pattern, no value
// pattern and no size bound. See docs/SCHEMAS.md.

// totalAnnotationSizeLimitB ports apimachinery's TotalAnnotationSizeLimitB
// (objectmeta.go): the combined size of all annotation keys and values.
const totalAnnotationSizeLimitB = 256 * (1 << 10) // 256 kB

// labelledMeta is the subset of metadata these checks read. Values are typed
// as map[string]string to match upstream; a manifest whose labels are not
// string-valued fails to unmarshal and is skipped rather than reported, since
// that shape is a schema error kubeconform already owns.
type labelledMeta struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

func parseLabelledMeta(data []byte) (kind string, meta labelledMeta, ok bool) {
	var doc struct {
		Kind     string       `json:"kind"`
		Metadata labelledMeta `json:"metadata"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return "", labelledMeta{}, false
	}
	if doc.Kind == "" {
		return "", labelledMeta{}, false
	}

	return doc.Kind, doc.Metadata, true
}

// sortedKeys returns map keys in a stable order so findings do not reorder
// between runs; Go map iteration is deliberately randomized.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return keys
}

// objectMetaLabelsInvalidCheck validates metadata.labels keys and values.
//
// Source: k8s.io/apimachinery/pkg/apis/meta/v1/validation/validation.go
// (ValidateLabels, ValidateLabelName)
type objectMetaLabelsInvalidCheck struct{}

func (c objectMetaLabelsInvalidCheck) ID() string { return "core/object-meta-labels-invalid" }
func (c objectMetaLabelsInvalidCheck) Title() string {
	return "Object Labels Must Be Valid"
}
func (c objectMetaLabelsInvalidCheck) Category() string      { return "core" }
func (c objectMetaLabelsInvalidCheck) Blocking() bool        { return true }
func (c objectMetaLabelsInvalidCheck) RenderSensitive() bool { return true }
func (c objectMetaLabelsInvalidCheck) Kinds() []string       { return nil }

func (c objectMetaLabelsInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	kind, meta, ok := parseLabelledMeta(data)
	if !ok || len(meta.Labels) == 0 {
		return nil
	}

	finding := func(message, value string) runtime.Finding {
		return runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:      "metadata.labels",
				Message:   message,
				Kind:      kind,
				Name:      meta.Name,
				Namespace: meta.Namespace,
				Value:     value,
			},
		}
	}

	var findings []runtime.Finding
	for _, key := range sortedKeys(meta.Labels) {
		// ValidateLabelName: the key is a qualified name, case-sensitive.
		for _, msg := range validation.IsQualifiedName(key) {
			findings = append(findings, finding(
				fmt.Sprintf("metadata.labels: invalid label key %q: %s", key, msg), key,
			))
		}
		// ValidateLabels: the value has its own, narrower rule.
		for _, msg := range validation.IsValidLabelValue(meta.Labels[key]) {
			findings = append(findings, finding(
				fmt.Sprintf("metadata.labels[%s]: invalid label value %q: %s", key, meta.Labels[key], msg),
				meta.Labels[key],
			))
		}
	}

	return findings
}

// objectMetaAnnotationsInvalidCheck validates metadata.annotation keys and the
// combined annotation size.
//
// Source: k8s.io/apimachinery/pkg/api/validation/objectmeta.go
// (ValidateAnnotations, ValidateAnnotationsSize)
type objectMetaAnnotationsInvalidCheck struct{}

func (c objectMetaAnnotationsInvalidCheck) ID() string {
	return "core/object-meta-annotations-invalid"
}

func (c objectMetaAnnotationsInvalidCheck) Title() string {
	return "Object Annotations Must Be Valid"
}
func (c objectMetaAnnotationsInvalidCheck) Category() string      { return "core" }
func (c objectMetaAnnotationsInvalidCheck) Blocking() bool        { return true }
func (c objectMetaAnnotationsInvalidCheck) RenderSensitive() bool { return true }
func (c objectMetaAnnotationsInvalidCheck) Kinds() []string       { return nil }

func (c objectMetaAnnotationsInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	kind, meta, ok := parseLabelledMeta(data)
	if !ok || len(meta.Annotations) == 0 {
		return nil
	}

	finding := func(message, value string) runtime.Finding {
		return runtime.Finding{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:      "metadata.annotations",
				Message:   message,
				Kind:      kind,
				Name:      meta.Name,
				Namespace: meta.Namespace,
				Value:     value,
			},
		}
	}

	var findings []runtime.Finding

	// Upstream lowercases the key before the qualified-name test, because
	// annotation keys are explicitly case-insensitive where label keys are
	// not. Dropping strings.ToLower here would reject the many real
	// annotations that carry uppercase characters.
	for _, key := range sortedKeys(meta.Annotations) {
		for _, msg := range validation.IsQualifiedName(strings.ToLower(key)) {
			findings = append(findings, finding(
				fmt.Sprintf("metadata.annotations: invalid annotation key %q: %s", key, msg), key,
			))
		}
	}

	// ValidateAnnotationsSize sums keys and values across all annotations and
	// compares the total against one shared limit, so this is reported once
	// for the object rather than per annotation.
	var totalSize int64
	for k, v := range meta.Annotations {
		totalSize += int64(len(k)) + int64(len(v))
	}
	if totalSize > int64(totalAnnotationSizeLimitB) {
		findings = append(findings, finding(
			fmt.Sprintf("metadata.annotations: too long: must have at most %d bytes", totalAnnotationSizeLimitB),
			"",
		))
	}

	return findings
}
