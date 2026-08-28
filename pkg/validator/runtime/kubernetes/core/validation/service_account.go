package validation

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

var serviceAccountKinds = []string{"ServiceAccount"}

type serviceAccountNameInvalidCheck struct{ coreNameCheckBase }

func (c serviceAccountNameInvalidCheck) ID() string { return "core/serviceaccount-name-invalid" }

func (c serviceAccountNameInvalidCheck) Title() string   { return "ServiceAccount Name Is Required" }
func (c serviceAccountNameInvalidCheck) Kinds() []string { return serviceAccountKinds }

func (c serviceAccountNameInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	return nameRequiredFindings(c, data, "ServiceAccount", "serviceaccount", func(data []byte) (string, bool) {
		var sa corev1.ServiceAccount
		if err := yaml.Unmarshal(data, &sa); err != nil {
			return "", false
		}
		return sa.GetName(), true
	})
}

func init() {
	registerNameRequiredCheck(serviceAccountNameInvalidCheck{})
}
