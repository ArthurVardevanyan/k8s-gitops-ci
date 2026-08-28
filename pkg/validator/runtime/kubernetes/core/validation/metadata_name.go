package validation

import (
	"sigs.k8s.io/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// coreNameCheckBase supplies the invariant Check metadata shared by the
// "metadata.name is required" checks in the core category.
type coreNameCheckBase struct{}

func (coreNameCheckBase) Category() string      { return "core" }
func (coreNameCheckBase) Blocking() bool        { return true }
func (coreNameCheckBase) RenderSensitive() bool { return true }

// registerNameRequiredCheck registers a single "metadata.name is required"
// check with the check registry.
func registerNameRequiredCheck(c runtime.Check) {
	check.Register(runtime.CheckToRegistered(c))
}

// nameRequiredFindings reports a finding when an object of the given kind
// decodes successfully but has no metadata.name. decode returns the object's
// name and whether the typed decode succeeded; messagePrefix is the lowercase
// object word used in the reported message.
func nameRequiredFindings(
	c runtime.Check,
	data []byte,
	kind string,
	messagePrefix string,
	decode func([]byte) (string, bool),
) []runtime.Finding {
	var ref struct {
		Kind string `json:"kind"`
	}
	if err := yaml.Unmarshal(data, &ref); err != nil || ref.Kind != kind {
		return nil
	}
	name, ok := decode(data)
	if !ok || name != "" {
		return nil
	}
	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Category:  c.Category(),
		Finding: check.Finding{
			Path:    "metadata.name",
			Message: messagePrefix + ": metadata.name is required",
			Kind:    kind,
		},
	}}
}
