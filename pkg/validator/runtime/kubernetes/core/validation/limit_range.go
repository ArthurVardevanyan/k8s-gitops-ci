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

var validLimitRangeTypes = map[corev1.LimitType]bool{
	corev1.LimitTypePod:                   true,
	corev1.LimitTypeContainer:             true,
	corev1.LimitTypePersistentVolumeClaim: true,
}

type limitRangeTypeInvalidCheck struct{}

func (c limitRangeTypeInvalidCheck) ID() string            { return "core/limitrange-type-invalid" }
func (c limitRangeTypeInvalidCheck) Title() string         { return "LimitRange Type Must Be Valid" }
func (c limitRangeTypeInvalidCheck) Category() string      { return "core" }
func (c limitRangeTypeInvalidCheck) Blocking() bool        { return true }
func (c limitRangeTypeInvalidCheck) RenderSensitive() bool { return true }
func (c limitRangeTypeInvalidCheck) DocSkipper() []string  { return limitRangeKinds }

func (c limitRangeTypeInvalidCheck) Run(data []byte, source string) []runtime.Finding {
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
		if item.Type == "" {
			continue
		}
		if !validLimitRangeTypes[item.Type] {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    field.NewPath("spec").Child("limits").Index(i).Child("type").String(),
					Message: fmt.Sprintf("limits[%d].type: unsupported value: %q: supported values: Pod, Container, PersistentVolumeClaim", i, string(item.Type)),
					Kind:    "LimitRange",
					Name:    lr.GetName(),
					Value:   string(item.Type),
				},
			})
		}
	}
	return findings
}

type limitRangeMaxInvalidCheck struct{}

func (c limitRangeMaxInvalidCheck) ID() string { return "core/limitrange-max-invalid" }
func (c limitRangeMaxInvalidCheck) Title() string {
	return "LimitRange Max Values Must Not Be Negative"
}
func (c limitRangeMaxInvalidCheck) Category() string      { return "core" }
func (c limitRangeMaxInvalidCheck) Blocking() bool        { return true }
func (c limitRangeMaxInvalidCheck) RenderSensitive() bool { return true }
func (c limitRangeMaxInvalidCheck) DocSkipper() []string  { return limitRangeKinds }

func (c limitRangeMaxInvalidCheck) Run(data []byte, source string) []runtime.Finding {
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
		if item.Max == nil {
			continue
		}
		for name, val := range item.Max {
			if val.Sign() < 0 {
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:    field.NewPath("spec").Child("limits").Index(i).Child("max").Key(string(name)).String(),
						Message: fmt.Sprintf("limits[%d].max.%s: %s must not be negative", i, name, val.String()),
						Kind:    "LimitRange",
						Name:    lr.GetName(),
						Value:   val.String(),
					},
				})
			}
		}
	}
	return findings
}

type limitRangeMinInvalidCheck struct{}

func (c limitRangeMinInvalidCheck) ID() string { return "core/limitrange-min-invalid" }
func (c limitRangeMinInvalidCheck) Title() string {
	return "LimitRange Min Values Must Not Be Negative"
}
func (c limitRangeMinInvalidCheck) Category() string      { return "core" }
func (c limitRangeMinInvalidCheck) Blocking() bool        { return true }
func (c limitRangeMinInvalidCheck) RenderSensitive() bool { return true }
func (c limitRangeMinInvalidCheck) DocSkipper() []string  { return limitRangeKinds }

func (c limitRangeMinInvalidCheck) Run(data []byte, source string) []runtime.Finding {
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
		if item.Min == nil {
			continue
		}
		for name, val := range item.Min {
			if val.Sign() < 0 {
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:    field.NewPath("spec").Child("limits").Index(i).Child("min").Key(string(name)).String(),
						Message: fmt.Sprintf("limits[%d].min.%s: %s must not be negative", i, name, val.String()),
						Kind:    "LimitRange",
						Name:    lr.GetName(),
						Value:   val.String(),
					},
				})
			}
		}
	}
	return findings
}

type limitRangeMaxMinInvalidCheck struct{}

func (c limitRangeMaxMinInvalidCheck) ID() string { return "core/limitrange-max-min-invalid" }

func (c limitRangeMaxMinInvalidCheck) Title() string         { return "LimitRange Max Must Be >= Min" }
func (c limitRangeMaxMinInvalidCheck) Category() string      { return "core" }
func (c limitRangeMaxMinInvalidCheck) Blocking() bool        { return true }
func (c limitRangeMaxMinInvalidCheck) RenderSensitive() bool { return true }
func (c limitRangeMaxMinInvalidCheck) DocSkipper() []string  { return limitRangeKinds }

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

type limitRangeNameInvalidCheck struct{}

func (c limitRangeNameInvalidCheck) ID() string            { return "core/limitrange-name-invalid" }
func (c limitRangeNameInvalidCheck) Title() string         { return "LimitRange Name Is Required" }
func (c limitRangeNameInvalidCheck) Category() string      { return "core" }
func (c limitRangeNameInvalidCheck) Blocking() bool        { return true }
func (c limitRangeNameInvalidCheck) RenderSensitive() bool { return true }
func (c limitRangeNameInvalidCheck) DocSkipper() []string  { return limitRangeKinds }

func (c limitRangeNameInvalidCheck) Run(data []byte, source string) []runtime.Finding {
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
	if lr.GetName() == "" {
		return []runtime.Finding{{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    "metadata.name",
				Message: "limitrange: metadata.name is required",
				Kind:    "LimitRange",
			},
		}}
	}
	return nil
}

type limitRangeDefaultInvalidCheck struct{}

func (c limitRangeDefaultInvalidCheck) ID() string { return "core/limitrange-default-invalid" }
func (c limitRangeDefaultInvalidCheck) Title() string {
	return "LimitRange Default Values Must Not Be Negative"
}
func (c limitRangeDefaultInvalidCheck) Category() string      { return "core" }
func (c limitRangeDefaultInvalidCheck) Blocking() bool        { return true }
func (c limitRangeDefaultInvalidCheck) RenderSensitive() bool { return true }
func (c limitRangeDefaultInvalidCheck) DocSkipper() []string  { return limitRangeKinds }

func (c limitRangeDefaultInvalidCheck) Run(data []byte, source string) []runtime.Finding {
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
		if item.Default == nil {
			continue
		}
		for name, val := range item.Default {
			if val.Sign() < 0 {
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:    field.NewPath("spec").Child("limits").Index(i).Child("default").Key(string(name)).String(),
						Message: fmt.Sprintf("limits[%d].default.%s: %s must not be negative", i, name, val.String()),
						Kind:    "LimitRange",
						Name:    lr.GetName(),
						Value:   val.String(),
					},
				})
			}
		}
	}
	return findings
}

type limitRangeDefaultRequestInvalidCheck struct{}

func (c limitRangeDefaultRequestInvalidCheck) ID() string {
	return "core/limitrange-default-request-invalid"
}

func (c limitRangeDefaultRequestInvalidCheck) Title() string {
	return "LimitRange DefaultRequest Values Must Not Be Negative"
}
func (c limitRangeDefaultRequestInvalidCheck) Category() string      { return "core" }
func (c limitRangeDefaultRequestInvalidCheck) Blocking() bool        { return true }
func (c limitRangeDefaultRequestInvalidCheck) RenderSensitive() bool { return true }
func (c limitRangeDefaultRequestInvalidCheck) DocSkipper() []string  { return limitRangeKinds }

func (c limitRangeDefaultRequestInvalidCheck) Run(data []byte, source string) []runtime.Finding {
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
		if item.DefaultRequest == nil {
			continue
		}
		for name, val := range item.DefaultRequest {
			if val.Sign() < 0 {
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:    field.NewPath("spec").Child("limits").Index(i).Child("defaultRequest").Key(string(name)).String(),
						Message: fmt.Sprintf("limits[%d].defaultRequest.%s: %s must not be negative", i, name, val.String()),
						Kind:    "LimitRange",
						Name:    lr.GetName(),
						Value:   val.String(),
					},
				})
			}
		}
	}
	return findings
}

func init() {
	checks := []runtime.Check{
		limitRangeTypeInvalidCheck{},
		limitRangeMaxInvalidCheck{},
		limitRangeMinInvalidCheck{},
		limitRangeMaxMinInvalidCheck{},
		limitRangeNameInvalidCheck{},
		limitRangeDefaultInvalidCheck{},
		limitRangeDefaultRequestInvalidCheck{},
	}
	for _, c := range checks {
		check.Register(runtime.CheckToRegistered(c))
	}
}
