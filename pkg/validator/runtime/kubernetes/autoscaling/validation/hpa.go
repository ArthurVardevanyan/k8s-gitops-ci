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
type maxReplicasInvalidCheck struct{}

func (c maxReplicasInvalidCheck) ID() string {
	return "autoscaling/max-replicas-invalid"
}

func (c maxReplicasInvalidCheck) Title() string {
	return "HPA maxReplicas Must Be Greater Than Zero"
}

func (c maxReplicasInvalidCheck) Category() string {
	return "autoscaling"
}

func (c maxReplicasInvalidCheck) Blocking() bool {
	return true
}

func (c maxReplicasInvalidCheck) RenderSensitive() bool {
	return true
}

func (c maxReplicasInvalidCheck) Kinds() []string {
	return hpaKinds
}

func (c maxReplicasInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	spec, err := parseHPA(data)
	if err != nil {
		return nil
	}
	name, _ := extractHPAName(data)

	val, found := extractNestedInt(spec, "maxReplicas")
	if !found {
		return []runtime.Finding{
			{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    field.NewPath("spec").Child("maxReplicas").String(),
					Message: "spec.maxReplicas: required",
					Kind:    "HorizontalPodAutoscaler",
					Name:    name,
				},
			},
		}
	}
	if val <= 0 {
		return []runtime.Finding{
			{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
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
type scaleDownInvalidCheck struct{}

func (c scaleDownInvalidCheck) ID() string {
	return "autoscaling/scale-down-invalid"
}

func (c scaleDownInvalidCheck) Title() string {
	return "HPA scaleDown stabilizationWindowSeconds Must Be >= 0"
}

func (c scaleDownInvalidCheck) Category() string {
	return "autoscaling"
}

func (c scaleDownInvalidCheck) Blocking() bool {
	return true
}

func (c scaleDownInvalidCheck) RenderSensitive() bool {
	return true
}

func (c scaleDownInvalidCheck) Kinds() []string {
	return hpaKinds
}

func (c scaleDownInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	return behaviorStabilizationWindowFindings(c, data, "scaleDown")
}

// scaleUpInvalidCheck validates scaleUp stabilizationWindowSeconds.
// Source: k8s.io/kubernetes/pkg/apis/autoscaling/validation/validation.go
type scaleUpInvalidCheck struct{}

func (c scaleUpInvalidCheck) ID() string {
	return "autoscaling/scale-up-invalid"
}

func (c scaleUpInvalidCheck) Title() string {
	return "HPA scaleUp stabilizationWindowSeconds Must Be >= 0"
}

func (c scaleUpInvalidCheck) Category() string {
	return "autoscaling"
}

func (c scaleUpInvalidCheck) Blocking() bool {
	return true
}

func (c scaleUpInvalidCheck) RenderSensitive() bool {
	return true
}

func (c scaleUpInvalidCheck) Kinds() []string {
	return hpaKinds
}

func (c scaleUpInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	return behaviorStabilizationWindowFindings(c, data, "scaleUp")
}

// behaviorStabilizationWindowFindings validates the
// stabilizationWindowSeconds entries under spec.behavior.<behaviorField>,
// which is identical logic for scaleUp and scaleDown.
// Source: k8s.io/kubernetes/pkg/apis/autoscaling/validation/validation.go
func behaviorStabilizationWindowFindings(c runtime.Check, data []byte, behaviorField string) []runtime.Finding {
	spec, err := parseHPA(data)
	if err != nil {
		return nil
	}
	name, _ := extractHPAName(data)

	entries, found := extractNestedSlice(spec, "behavior", behaviorField)
	if !found {
		return nil
	}

	var findings []runtime.Finding
	for i, entry := range entries {
		entryPath := field.NewPath("spec").Child("behavior").Child(behaviorField).Index(i)
		if val, ok := extractNestedInt(entry, "stabilizationWindowSeconds"); ok {
			if val < 0 {
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:    entryPath.Child("stabilizationWindowSeconds").String(),
						Message: fmt.Sprintf("behavior.%s[%d].stabilizationWindowSeconds: invalid value: %d: must be >= 0", behaviorField, i, val),
						Kind:    "HorizontalPodAutoscaler",
						Name:    name,
						Value:   fmt.Sprintf("%d", val),
					},
				})
			}
		}
	}
	return findings
}

// init registers all HPA validation checks (applies to both v1 and v2 API versions).
func init() {
	checks := []runtime.Check{
		maxReplicasInvalidCheck{},
		scaleDownInvalidCheck{},
		scaleUpInvalidCheck{},
	}
	for _, c := range checks {
		check.Register(runtime.CheckToRegistered(c))
	}
}
