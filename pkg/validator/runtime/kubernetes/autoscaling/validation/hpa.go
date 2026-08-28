package validation

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

var hpaKinds = []string{"HorizontalPodAutoscaler"}

var validHPAKinds = map[string]bool{
	"ReplicationController": true,
	"ReplicaSet":            true,
	"Deployment":            true,
	"StatefulSet":           true,
	"DaemonSet":             true,
}

var validMetricTypes = map[string]bool{
	"Utilization":  true,
	"AverageValue": true,
	"Value":        true,
}

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

func extractNestedString(data map[string]interface{}, path ...string) (string, bool) {
	if len(path) == 0 {
		return "", false
	}
	cur := data
	for _, p := range path[:len(path)-1] {
		next, ok := cur[p].(map[string]interface{})
		if !ok {
			return "", false
		}
		cur = next
	}
	v, ok := cur[path[len(path)-1]].(string)
	return v, ok
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

func (c maxReplicasInvalidCheck) DocSkipper() []string {
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

// minReplicasInvalidCheck validates minReplicas >= 0 and minReplicas <= maxReplicas.
// Source: k8s.io/kubernetes/pkg/apis/autoscaling/validation/validation.go
type minReplicasInvalidCheck struct{}

func (c minReplicasInvalidCheck) ID() string {
	return "autoscaling/min-replicas-invalid"
}

func (c minReplicasInvalidCheck) Title() string {
	return "HPA minReplicas Must Be >= 0 and <= maxReplicas"
}

func (c minReplicasInvalidCheck) Category() string {
	return "autoscaling"
}

func (c minReplicasInvalidCheck) Blocking() bool {
	return true
}

func (c minReplicasInvalidCheck) RenderSensitive() bool {
	return true
}

func (c minReplicasInvalidCheck) DocSkipper() []string {
	return hpaKinds
}

func (c minReplicasInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	spec, err := parseHPA(data)
	if err != nil {
		return nil
	}
	name, _ := extractHPAName(data)

	minVal, minFound := extractNestedInt(spec, "minReplicas")
	maxVal, maxFound := extractNestedInt(spec, "maxReplicas")

	if minFound {
		if !maxFound {
			// maxReplicas missing — already flagged by maxReplicasInvalidCheck
			return nil
		}
		if minVal < 0 {
			return []runtime.Finding{
				{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:    field.NewPath("spec").Child("minReplicas").String(),
						Message: fmt.Sprintf("spec.minReplicas: invalid value: %d: must be >= 0", minVal),
						Kind:    "HorizontalPodAutoscaler",
						Name:    name,
						Value:   fmt.Sprintf("%d", minVal),
					},
				},
			}
		}
		if minVal > maxVal {
			return []runtime.Finding{
				{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:    field.NewPath("spec").Child("minReplicas").String(),
						Message: fmt.Sprintf("spec.minReplicas: invalid value: %d: must be <= %d (maxReplicas)", minVal, maxVal),
						Kind:    "HorizontalPodAutoscaler",
						Name:    name,
						Value:   fmt.Sprintf("%d", minVal),
					},
				},
			}
		}
	}
	return nil
}

// scaleTargetRefInvalidCheck validates scaleTargetRef.kind.
// Source: k8s.io/kubernetes/pkg/apis/autoscaling/validation/validation.go
type scaleTargetRefInvalidCheck struct{}

func (c scaleTargetRefInvalidCheck) ID() string {
	return "autoscaling/scale-target-ref-invalid"
}

func (c scaleTargetRefInvalidCheck) Title() string {
	return "HPA scaleTargetRef Kind Must Be Valid"
}

func (c scaleTargetRefInvalidCheck) Category() string {
	return "autoscaling"
}

func (c scaleTargetRefInvalidCheck) Blocking() bool {
	return true
}

func (c scaleTargetRefInvalidCheck) RenderSensitive() bool {
	return true
}

func (c scaleTargetRefInvalidCheck) DocSkipper() []string {
	return hpaKinds
}

func (c scaleTargetRefInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	spec, err := parseHPA(data)
	if err != nil {
		return nil
	}
	name, _ := extractHPAName(data)

	kind, found := extractNestedString(spec, "scaleTargetRef", "kind")
	if !found {
		return nil
	}
	if !validHPAKinds[kind] {
		return []runtime.Finding{
			{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    field.NewPath("spec").Child("scaleTargetRef").Child("kind").String(),
					Message: fmt.Sprintf("scaleTargetRef.kind: Unsupported value: %q: supported values: \"ReplicationController\", \"ReplicaSet\", \"Deployment\", \"StatefulSet\", \"DaemonSet\"", kind),
					Kind:    "HorizontalPodAutoscaler",
					Name:    name,
					Value:   kind,
				},
			},
		}
	}
	return nil
}

// metricSpecInvalidCheck validates metricSpec entries (target type, metric name, target value).
// Source: k8s.io/kubernetes/pkg/apis/autoscaling/validation/validation.go
type metricSpecInvalidCheck struct{}

func (c metricSpecInvalidCheck) ID() string {
	return "autoscaling/metric-spec-invalid"
}

func (c metricSpecInvalidCheck) Title() string {
	return "HPA metricSpec Must Be Valid"
}

func (c metricSpecInvalidCheck) Category() string {
	return "autoscaling"
}

func (c metricSpecInvalidCheck) Blocking() bool {
	return true
}

func (c metricSpecInvalidCheck) RenderSensitive() bool {
	return true
}

func (c metricSpecInvalidCheck) DocSkipper() []string {
	return hpaKinds
}

func (c metricSpecInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	spec, err := parseHPA(data)
	if err != nil {
		return nil
	}
	name, _ := extractHPAName(data)

	metrics, found := extractNestedSlice(spec, "metrics")
	if !found {
		return nil
	}

	var findings []runtime.Finding
	for i, metric := range metrics {
		metricPath := field.NewPath("spec").Child("metrics").Index(i)

		// Check target type
		targetType, ttFound := extractNestedString(metric, "type")
		if !ttFound {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    metricPath.Child("type").String(),
					Message: "metrics[].type: required",
					Kind:    "HorizontalPodAutoscaler",
					Name:    name,
				},
			})
			continue
		}
		if !validMetricTypes[targetType] {
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:    metricPath.Child("type").String(),
					Message: fmt.Sprintf("metrics[%d].type: Unsupported value: %q", i, targetType),
					Kind:    "HorizontalPodAutoscaler",
					Name:    name,
					Value:   targetType,
				},
			})
		}

		// Check target value based on type
		target := metric["target"]
		if targetMap, ok := target.(map[string]interface{}); ok {
			switch targetType {
			case "Utilization":
				// TargetUtilizationPercentage
				if val, ok := extractNestedInt(targetMap, "targetUtilizationPercentage"); ok {
					if val <= 0 {
						findings = append(findings, runtime.Finding{
							RuleID:    c.ID(),
							RuleTitle: c.Title(),
							Category:  c.Category(),
							Finding: check.Finding{
								Path:    metricPath.Child("target").Child("targetUtilizationPercentage").String(),
								Message: fmt.Sprintf("metrics[%d].target.targetUtilizationPercentage: invalid value: %d: must be > 0", i, val),
								Kind:    "HorizontalPodAutoscaler",
								Name:    name,
								Value:   fmt.Sprintf("%d", val),
							},
						})
					}
				}
			case "AverageValue", "Value":
				if val, ok := extractNestedInt(targetMap, targetType); ok {
					if val <= 0 {
						findings = append(findings, runtime.Finding{
							RuleID:    c.ID(),
							RuleTitle: c.Title(),
							Category:  c.Category(),
							Finding: check.Finding{
								Path:    metricPath.Child("target").Child(targetType).String(),
								Message: fmt.Sprintf("metrics[%d].target.%s: invalid value: %d: must be > 0", i, targetType, val),
								Kind:    "HorizontalPodAutoscaler",
								Name:    name,
								Value:   fmt.Sprintf("%d", val),
							},
						})
					}
				}
			}
		}
	}

	return findings
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

func (c scaleDownInvalidCheck) DocSkipper() []string {
	return hpaKinds
}

func (c scaleDownInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	spec, err := parseHPA(data)
	if err != nil {
		return nil
	}
	name, _ := extractHPAName(data)

	scaleDown, found := extractNestedSlice(spec, "behavior", "scaleDown")
	if !found {
		return nil
	}

	var findings []runtime.Finding
	for i, entry := range scaleDown {
		sdPath := field.NewPath("spec").Child("behavior").Child("scaleDown").Index(i)
		if val, ok := extractNestedInt(entry, "stabilizationWindowSeconds"); ok {
			if val < 0 {
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:    sdPath.Child("stabilizationWindowSeconds").String(),
						Message: fmt.Sprintf("behavior.scaleDown[%d].stabilizationWindowSeconds: invalid value: %d: must be >= 0", i, val),
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

func (c scaleUpInvalidCheck) DocSkipper() []string {
	return hpaKinds
}

func (c scaleUpInvalidCheck) Run(data []byte, source string) []runtime.Finding {
	spec, err := parseHPA(data)
	if err != nil {
		return nil
	}
	name, _ := extractHPAName(data)

	scaleUp, found := extractNestedSlice(spec, "behavior", "scaleUp")
	if !found {
		return nil
	}

	var findings []runtime.Finding
	for i, entry := range scaleUp {
		suPath := field.NewPath("spec").Child("behavior").Child("scaleUp").Index(i)
		if val, ok := extractNestedInt(entry, "stabilizationWindowSeconds"); ok {
			if val < 0 {
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:    suPath.Child("stabilizationWindowSeconds").String(),
						Message: fmt.Sprintf("behavior.scaleUp[%d].stabilizationWindowSeconds: invalid value: %d: must be >= 0", i, val),
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
		minReplicasInvalidCheck{},
		scaleTargetRefInvalidCheck{},
		metricSpecInvalidCheck{},
		scaleDownInvalidCheck{},
		scaleUpInvalidCheck{},
	}
	for _, c := range checks {
		check.Register(runtime.CheckToRegistered(c))
	}
}
