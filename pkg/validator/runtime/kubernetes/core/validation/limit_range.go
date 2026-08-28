package validation

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

var limitRangeKinds = []string{"LimitRange"}

type limitRangeMaxMinInvalidCheck struct{}

func (c limitRangeMaxMinInvalidCheck) ID() string { return "core/limitrange-max-min-invalid" }

func (c limitRangeMaxMinInvalidCheck) Title() string         { return "LimitRange Max Must Be >= Min" }
func (c limitRangeMaxMinInvalidCheck) Category() string      { return "core" }
func (c limitRangeMaxMinInvalidCheck) Blocking() bool        { return true }
func (c limitRangeMaxMinInvalidCheck) RenderSensitive() bool { return true }
func (c limitRangeMaxMinInvalidCheck) Kinds() []string       { return limitRangeKinds }

func (c limitRangeMaxMinInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	var ref struct {
		Kind string `json:"kind"`
	}
	if err := yaml.Unmarshal(data, &ref); err != nil || ref.Kind != "LimitRange" {
		return nil
	}
	var lr corev1.LimitRange
	if err := yaml.Unmarshal(data, &lr); err != nil {
		return nil
	}
	var findings []runtime.Finding
	for i, item := range lr.Spec.Limits {
		if item.Max == nil || item.Min == nil {
			continue
		}
		for name, minVal := range item.Min {
			maxVal, ok := item.Max[name]
			if !ok {
				continue
			}
			if maxVal.Cmp(minVal) < 0 {
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:    field.NewPath("spec").Child("limits").Index(i).Child("max").Key(string(name)).String(),
						Message: fmt.Sprintf("limits[%d]: max.%s (%s) must be >= min.%s (%s)", i, name, maxVal.String(), name, minVal.String()),
						Kind:    "LimitRange",
						Name:    lr.GetName(),
					},
				})
			}
		}
	}
	return findings
}
