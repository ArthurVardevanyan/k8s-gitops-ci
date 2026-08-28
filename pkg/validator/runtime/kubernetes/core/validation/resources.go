package validation

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// resourceRequestsGreaterThanLimitsCheck ensures that resource requests are
// less than or equal to limits for each resource type.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type resourceRequestsGreaterThanLimitsCheck struct{}

func (c resourceRequestsGreaterThanLimitsCheck) ID() string {
	return "resources/resource-requests-greater-than-limits"
}

func (c resourceRequestsGreaterThanLimitsCheck) Title() string {
	return "Resource Requests Must Not Exceed Limits"
}

func (c resourceRequestsGreaterThanLimitsCheck) Category() string {
	return "resources"
}

func (c resourceRequestsGreaterThanLimitsCheck) Blocking() bool {
	return true
}

func (c resourceRequestsGreaterThanLimitsCheck) RenderSensitive() bool {
	return true
}

func (c resourceRequestsGreaterThanLimitsCheck) Kinds() []string {
	return runtime.HasPodSpecKinds()
}

func (c resourceRequestsGreaterThanLimitsCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	containers := runtime.AllContainers(info)

	for _, ctr := range containers {
		resPath := ctr.Path.Child("resources")
		res := ctr.Container.Resources

		if len(res.Requests) == 0 && len(res.Limits) == 0 {
			continue
		}

		requests := res.Requests
		limits := res.Limits

		if len(requests) == 0 || len(limits) == 0 {
			continue
		}

		for resName, reqVal := range requests {
			limitVal, hasLimit := limits[resName]
			if !hasLimit {
				continue
			}

			if reqVal.Cmp(limitVal) > 0 {
				reqStr := reqVal.String()
				limitStr := limitVal.String()
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:      resPath.Child("requests").Child(string(resName)).String(),
						Message:   fmt.Sprintf("container %q: %s request %s must be less than or equal to limit %s", ctr.Container.Name, resName, reqStr, limitStr),
						Kind:      info.Kind,
						Name:      info.Name,
						Namespace: info.Namespace,
						Container: ctr.Container.Name,
						Value:     fmt.Sprintf("%s > %s", reqStr, limitStr),
					},
				})
			}
		}
	}

	return findings
}

// resourceQuantityNegativeCheck ensures that resource quantities are not
// negative. This check catches quantities that compare as less than zero.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go
type resourceQuantityNegativeCheck struct{}

func (c resourceQuantityNegativeCheck) ID() string {
	return "resources/resource-quantity-negative"
}

func (c resourceQuantityNegativeCheck) Title() string {
	return "Resource Quantity Must Not Be Negative"
}

func (c resourceQuantityNegativeCheck) Category() string {
	return "resources"
}

func (c resourceQuantityNegativeCheck) Blocking() bool {
	return true
}

func (c resourceQuantityNegativeCheck) RenderSensitive() bool {
	return true
}

func (c resourceQuantityNegativeCheck) Kinds() []string {
	return runtime.HasPodSpecKinds()
}

func (c resourceQuantityNegativeCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	containers := runtime.AllContainers(info)

	for _, ctr := range containers {
		resPath := ctr.Path.Child("resources")
		res := ctr.Container.Resources

		if len(res.Requests) == 0 && len(res.Limits) == 0 {
			continue
		}

		checkResourceQuantity := func(reqList corev1.ResourceList, path *field.Path) {
			for resName, qty := range reqList {
				if qty.Sign() < 0 {
					qtyStr := qty.String()
					findings = append(findings, runtime.Finding{
						RuleID:    c.ID(),
						RuleTitle: c.Title(),
						Category:  c.Category(),
						Finding: check.Finding{
							Path:      path.Child(string(resName)).String(),
							Message:   fmt.Sprintf("container %q: %s: resource quantity must not be negative", ctr.Container.Name, resName),
							Kind:      info.Kind,
							Name:      info.Name,
							Namespace: info.Namespace,
							Container: ctr.Container.Name,
							Value:     qtyStr,
						},
					})
				}
			}
		}

		if res.Requests != nil {
			checkResourceQuantity(res.Requests, resPath.Child("requests"))
		}
		if res.Limits != nil {
			checkResourceQuantity(res.Limits, resPath.Child("limits"))
		}
	}

	return findings
}

// isHugePageResource returns true if the resource name is a huge page type.
func isHugePageResource(name corev1.ResourceName) bool {
	s := string(name)
	return len(s) > 9 && s[:9] == "hugepages"
}
