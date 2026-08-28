package validation

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
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

type configMapDataSizeExceededCheck struct{ runtime.Meta }

func newConfigMapDataSizeExceededCheck() configMapDataSizeExceededCheck {
	return configMapDataSizeExceededCheck{runtime.Meta{
		RuleID:    "core/configmap-data-size-exceeded",
		RuleTitle: "ConfigMap Data Size Must Not Exceed 1 MiB",
		AppliesTo: configMapKinds,
	}}
}

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
