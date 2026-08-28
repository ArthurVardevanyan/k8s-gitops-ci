package validation

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

var configMapKinds = []string{"ConfigMap"}

func isKind(data []byte, kind string) bool {
	var ref struct {
		Kind string `json:"kind"`
	}
	if err := yaml.Unmarshal(data, &ref); err != nil {
		return false
	}
	return ref.Kind == kind
}

type configMapDataInvalidKeyCheck struct{}

func (c configMapDataInvalidKeyCheck) ID() string { return "core/configmap-data-invalid-key" }

func (c configMapDataInvalidKeyCheck) Title() string         { return "ConfigMap Data Keys Must Be Valid" }
func (c configMapDataInvalidKeyCheck) Category() string      { return "core" }
func (c configMapDataInvalidKeyCheck) Blocking() bool        { return true }
func (c configMapDataInvalidKeyCheck) RenderSensitive() bool { return true }
func (c configMapDataInvalidKeyCheck) DocSkipper() []string  { return configMapKinds }

func (c configMapDataInvalidKeyCheck) Run(data []byte, source string) []runtime.Finding {
	if !isKind(data, "ConfigMap") {
		return nil
	}
	var cm corev1.ConfigMap
	if err := yaml.Unmarshal(data, &cm); err != nil {
		return nil
	}
	var findings []runtime.Finding
	for k := range cm.Data {
		if errors := validation.IsQualifiedName(k); len(errors) > 0 {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    field.NewPath("data").Key(k).String(),
					Message: fmt.Sprintf("data: invalid key: %s", errors[0]),
					Kind:    "ConfigMap",
					Name:    cm.GetName(),
				},
			})
		}
	}
	for k := range cm.BinaryData {
		if errors := validation.IsQualifiedName(k); len(errors) > 0 {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    field.NewPath("binaryData").Key(k).String(),
					Message: fmt.Sprintf("binaryData: invalid key: %s", errors[0]),
					Kind:    "ConfigMap",
					Name:    cm.GetName(),
				},
			})
		}
	}
	return findings
}

type configMapDataSizeExceededCheck struct{}

func (c configMapDataSizeExceededCheck) ID() string { return "core/configmap-data-size-exceeded" }
func (c configMapDataSizeExceededCheck) Title() string {
	return "ConfigMap Data Size Must Not Exceed 1 MiB"
}
func (c configMapDataSizeExceededCheck) Category() string      { return "core" }
func (c configMapDataSizeExceededCheck) Blocking() bool        { return true }
func (c configMapDataSizeExceededCheck) RenderSensitive() bool { return true }
func (c configMapDataSizeExceededCheck) DocSkipper() []string  { return configMapKinds }

const maxConfigMapSize = 1048576

func (c configMapDataSizeExceededCheck) Run(data []byte, source string) []runtime.Finding {
	if !isKind(data, "ConfigMap") {
		return nil
	}
	var cm corev1.ConfigMap
	if err := yaml.Unmarshal(data, &cm); err != nil {
		return nil
	}
	var totalSize int
	for _, v := range cm.Data {
		totalSize += len(v)
	}
	for _, v := range cm.BinaryData {
		totalSize += len(v)
	}
	if totalSize > maxConfigMapSize {
		return []runtime.Finding{{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    "data",
				Message: fmt.Sprintf("configmap %q: data/binaryData total size %d exceeds limit of %d bytes (1 MiB)", cm.GetName(), totalSize, maxConfigMapSize),
				Kind:    "ConfigMap",
				Name:    cm.GetName(),
			},
		}}
	}
	return nil
}

type configMapNameInvalidCheck struct{}

func (c configMapNameInvalidCheck) ID() string            { return "core/configmap-name-invalid" }
func (c configMapNameInvalidCheck) Title() string         { return "ConfigMap Name Is Required" }
func (c configMapNameInvalidCheck) Category() string      { return "core" }
func (c configMapNameInvalidCheck) Blocking() bool        { return true }
func (c configMapNameInvalidCheck) RenderSensitive() bool { return true }
func (c configMapNameInvalidCheck) DocSkipper() []string  { return configMapKinds }

func (c configMapNameInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	if !isKind(data, "ConfigMap") {
		return nil
	}
	var cm corev1.ConfigMap
	if err := yaml.Unmarshal(data, &cm); err != nil {
		return nil
	}
	if cm.GetName() == "" {
		return []runtime.Finding{{
			RuleID:    c.ID(),
			RuleTitle: c.Title(),
			Category:  c.Category(),
			Finding: check.Finding{
				Path:    "metadata.name",
				Message: "configmap: metadata.name is required",
				Kind:    "ConfigMap",
			},
		}}
	}
	return nil
}

func init() {
	checks := []runtime.Check{
		configMapDataInvalidKeyCheck{},
		configMapDataSizeExceededCheck{},
		configMapNameInvalidCheck{},
	}
	for _, c := range checks {
		check.Register(runtime.CheckToRegistered(c))
	}
}
