package validation

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

var secretKinds = []string{"Secret"}

type secretNameInvalidCheck struct{ coreNameCheckBase }

func (c secretNameInvalidCheck) ID() string      { return "core/secret-name-invalid" }
func (c secretNameInvalidCheck) Title() string   { return "Secret Name Is Required" }
func (c secretNameInvalidCheck) Kinds() []string { return secretKinds }

func (c secretNameInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	return nameRequiredFindings(c, data, "Secret", "secret", func(data []byte) (string, bool) {
		var s corev1.Secret
		if err := yaml.Unmarshal(data, &s); err != nil {
			return "", false
		}
		return s.GetName(), true
	})
}

func init() {
	registerNameRequiredCheck(secretNameInvalidCheck{})
}
