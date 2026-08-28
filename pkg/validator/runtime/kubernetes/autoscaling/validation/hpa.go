package validation

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

var hpaKinds = []string{"HorizontalPodAutoscaler"}

func extractHPAName(data []byte) (string, error) {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return "", err
	}
	meta, ok := raw["metadata"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("no metadata")
	}
	name, _ := meta["name"].(string)
	return name, nil
}

func extractNestedInt(data map[string]interface{}, path ...string) (int64, bool) {
	if len(path) == 0 {
		return 0, false
	}
	cur := data
	for _, p := range path[:len(path)-1] {
		next, ok := cur[p].(map[string]interface{})
		if !ok {
			return 0, false
		}
		cur = next
	}
	if num, ok := cur[path[len(path)-1]].(float64); ok {
		return int64(num), true
	}
	return 0, false
}

func extractNestedSlice(data map[string]interface{}, path ...string) ([]map[string]interface{}, bool) {
	if len(path) == 0 {
		return nil, false
	}
	cur := data
	for _, p := range path[:len(path)-1] {
		next, ok := cur[p].(map[string]interface{})
		if !ok {
			return nil, false
		}
		cur = next
	}
	raw := cur[path[len(path)-1]]
	// Handle single map entry (YAML: key: {...})
	if m, ok := raw.(map[string]interface{}); ok {
		return []map[string]interface{}{m}, true
	}
	// Handle array of maps (YAML: key: [{...}, {...}])
	arr, ok := raw.([]interface{})
	if !ok {
		return nil, false
	}
	result := make([]map[string]interface{}, 0, len(arr))
	for _, item := range arr {
		if m, ok := item.(map[string]interface{}); ok {
			result = append(result, m)
		}
	}
	return result, true
}

func parseHPA(data []byte) (map[string]interface{}, error) {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	spec, ok := raw["spec"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("no spec")
	}
	return spec, nil
}

// maxReplicasInvalidCheck validates that maxReplicas > 0.
// Source: k8s.io/kubernetes/pkg/apis/autoscaling/validation/validation.go
type maxReplicasInvalidCheck struct{ runtime.Meta }

func newMaxReplicasInvalidCheck() maxReplicasInvalidCheck {
	return maxReplicasInvalidCheck{runtime.Meta{
		RuleID:    "autoscaling/max-replicas-invalid",
		RuleTitle: "HPA maxReplicas Must Be Greater Than Zero",
		AppliesTo: hpaKinds,
	}}
}

func (c maxReplicasInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	spec, err := parseHPA(data)
	if err != nil {
		return nil
	}
	name, _ := extractHPAName(data)

	val, found := extractNestedInt(spec, "maxReplicas")
	if !found {
		// A missing spec.maxReplicas is already rejected by kubeconform:
		// maxReplicas is in the `required` array of every HPA schema
		// variant (autoscaling/v1, autoscaling/v2, and the default). This
		// is the only place in the runtime family where the JSON schema
		// fully subsumes a branch, because it is the only branch keyed on
		// key *presence* rather than on a value the schema cannot
		// constrain. Reporting it here too would just double-report.
		// See docs/SCHEMAS.md.
		return nil
	}
	if val <= 0 {
		return []runtime.Finding{
			{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Finding: check.Finding{
					Path:    field.NewPath("spec").Child("maxReplicas").String(),
					Message: fmt.Sprintf("spec.maxReplicas: invalid value: %d: must be greater than 0", val),
					Kind:    "HorizontalPodAutoscaler",
					Name:    name,
					Value:   fmt.Sprintf("%d", val),
				},
			},
		}
	}
	return nil
}

// scaleDownInvalidCheck validates scaleDown stabilizationWindowSeconds.
// Source: k8s.io/kubernetes/pkg/apis/autoscaling/validation/validation.go
type scaleDownInvalidCheck struct{ runtime.Meta }

func newScaleDownInvalidCheck() scaleDownInvalidCheck {
	return scaleDownInvalidCheck{runtime.Meta{
		RuleID:    "autoscaling/scale-down-invalid",
		RuleTitle: "HPA scaleDown stabilizationWindowSeconds Must Be >= 0",
		AppliesTo: hpaKinds,
	}}
}

func (c scaleDownInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	return behaviorStabilizationWindowFindings(c, data, "scaleDown")
}

// scaleUpInvalidCheck validates scaleUp stabilizationWindowSeconds.
// Source: k8s.io/kubernetes/pkg/apis/autoscaling/validation/validation.go
type scaleUpInvalidCheck struct{ runtime.Meta }

func newScaleUpInvalidCheck() scaleUpInvalidCheck {
	return scaleUpInvalidCheck{runtime.Meta{
		RuleID:    "autoscaling/scale-up-invalid",
		RuleTitle: "HPA scaleUp stabilizationWindowSeconds Must Be >= 0",
		AppliesTo: hpaKinds,
	}}
}

func (c scaleUpInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	return behaviorStabilizationWindowFindings(c, data, "scaleUp")
}

// behaviorStabilizationWindowFindings validates the
// stabilizationWindowSeconds entries under spec.behavior.<behaviorField>,
// which is identical logic for scaleUp and scaleDown.
// Source: k8s.io/kubernetes/pkg/apis/autoscaling/validation/validation.go
// behaviorStabilizationWindowFindings validates
// behavior.<scaleUp|scaleDown>.stabilizationWindowSeconds.
//
// Both are HPAScalingRules objects, not lists, so the finding path carries
// no index: spec.behavior.scaleDown[0].stabilizationWindowSeconds is not a
// field an HPA manifest has.
func behaviorStabilizationWindowFindings(c runtime.Check, data []byte, behaviorField string) []runtime.Finding {
	spec, err := parseHPA(data)
	if err != nil {
		return nil
	}
	name, _ := extractHPAName(data)

	behavior, ok := spec["behavior"].(map[string]interface{})
	if !ok {
		return nil
	}
	rules, ok := behavior[behaviorField].(map[string]interface{})
	if !ok {
		return nil
	}

	val, ok := extractNestedInt(rules, "stabilizationWindowSeconds")
	if !ok || val >= 0 {
		return nil
	}

	return []runtime.Finding{{
		RuleID:    c.ID(),
		RuleTitle: c.Title(),
		Finding: check.Finding{
			Path:    field.NewPath("spec").Child("behavior").Child(behaviorField).Child("stabilizationWindowSeconds").String(),
			Message: fmt.Sprintf("behavior.%s.stabilizationWindowSeconds: invalid value: %d: must be >= 0", behaviorField, val),
			Kind:    "HorizontalPodAutoscaler",
			Name:    name,
			Value:   fmt.Sprintf("%d", val),
		},
	}}
}

// init registers all HPA validation checks (applies to both v1 and v2 API versions).
//
// Registration funnels through runtime.RegisterAll so that each check must
// carry an UpstreamRef in upstreamRefs citing the exact upstream Kubernetes
// function it ports; RegisterAll panics on a check with no valid citation.
func init() {
	checks := []runtime.Check{
		newMaxReplicasInvalidCheck(),
		newScaleDownInvalidCheck(),
		newScaleUpInvalidCheck(),
	}

	runtime.RegisterAll(checks, upstreamRefs)
}
