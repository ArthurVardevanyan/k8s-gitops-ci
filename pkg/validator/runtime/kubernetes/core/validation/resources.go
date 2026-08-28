package validation

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// resourceQuantityFormatCheck validates that resource quantities are valid.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go:5200-5210
type resourceQuantityFormatCheck struct{}

func (c resourceQuantityFormatCheck) ID() string {
	return "resources/resource-quantity-format"
}

func (c resourceQuantityFormatCheck) Title() string {
	return "Resource Quantity Must Be Valid"
}

func (c resourceQuantityFormatCheck) Category() string {
	return "resources"
}

func (c resourceQuantityFormatCheck) Blocking() bool {
	return true
}

func (c resourceQuantityFormatCheck) RenderSensitive() bool {
	return true
}

func (c resourceQuantityFormatCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c resourceQuantityFormatCheck) Run(data []byte, source string) []runtime.Finding {
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

		// Check all keys in requests and limits for valid quantities.
		// Since the Kubernetes API server validates quantities during
		// unmarshaling, by the time we reach this check the values here
		// are already parsed. However, we still validate to catch edge
		// cases in custom/overlaid manifests that may have bypassed
		// standard parsing.
		allRes := make(corev1.ResourceList)
		for k, v := range res.Requests {
			allRes[k] = v
		}
		for k, v := range res.Limits {
			allRes[k] = v
		}

		for resName, quantity := range allRes {
			if validateResourceQuantity(resName, quantity) == "" {
				continue
			}
			err := validateResourceQuantity(resName, quantity)
			isRequest := res.Requests != nil && res.Requests[resName] == quantity
			path := resPath.Child("requests").Child(string(resName))
			if !isRequest {
				path = resPath.Child("limits").Child(string(resName))
			}
			qtyStr := quantity.String()
			findings = append(findings, runtime.Finding{
				RuleID:    c.ID(),
				RuleTitle: c.Title(),
				Category:  c.Category(),
				Finding: check.Finding{
					Path:      path.String(),
					Message:   fmt.Sprintf("container %q: %s: %s", ctr.Container.Name, resName, err),
					Kind:      info.Kind,
					Name:      info.Name,
					Namespace: info.Namespace,
					Container: ctr.Container.Name,
					Value:     qtyStr,
				},
			})
		}
	}

	return findings
}

// resourceLimitsMissingCheck ensures that if requests are set, all requested
// resources must have a corresponding limit (and vice versa).
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go:5200-5210
type resourceLimitsMissingCheck struct{}

func (c resourceLimitsMissingCheck) ID() string {
	return "resources/resource-limits-missing"
}

func (c resourceLimitsMissingCheck) Title() string {
	return "Resource Limits Must Match Requests"
}

func (c resourceLimitsMissingCheck) Category() string {
	return "resources"
}

func (c resourceLimitsMissingCheck) Blocking() bool {
	return true
}

func (c resourceLimitsMissingCheck) RenderSensitive() bool {
	return true
}

func (c resourceLimitsMissingCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c resourceLimitsMissingCheck) Run(data []byte, source string) []runtime.Finding {
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

		// Check which requests don't have corresponding limits
		if len(requests) > 0 {
			for resName := range requests {
				if _, hasLimit := limits[resName]; !hasLimit {
					qty := requests[resName]
					qtyStr := qty.String()
					findings = append(findings, runtime.Finding{
						RuleID:    c.ID(),
						RuleTitle: c.Title(),
						Category:  c.Category(),
						Finding: check.Finding{
							Path:      resPath.Child("requests").Child(string(resName)).String(),
							Message:   fmt.Sprintf("container %q: when requests are set, all requests must have a corresponding limit: missing limit for %s", ctr.Container.Name, resName),
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

		// Check which limits don't have corresponding requests
		if len(limits) > 0 {
			for resName := range limits {
				if _, hasRequest := requests[resName]; !hasRequest {
					qty := limits[resName]
					qtyStr := qty.String()
					findings = append(findings, runtime.Finding{
						RuleID:    c.ID(),
						RuleTitle: c.Title(),
						Category:  c.Category(),
						Finding: check.Finding{
							Path:      resPath.Child("limits").Child(string(resName)).String(),
							Message:   fmt.Sprintf("container %q: when limits are set, all limits must have a corresponding request: missing request for %s", ctr.Container.Name, resName),
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
	}

	return findings
}

// resourceRequestsGreaterThanLimitsCheck ensures that resource requests are
// less than or equal to limits for each resource type.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go:5200-5210
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

func (c resourceRequestsGreaterThanLimitsCheck) DocSkipper() []string {
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

// hugepagesInRequestsCheck ensures that huge pages are only specified in limits,
// not in requests (Kubernetes-enforced rule).
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go:5200-5210
type hugepagesInRequestsCheck struct{}

func (c hugepagesInRequestsCheck) ID() string {
	return "resources/hugepages-in-requests"
}

func (c hugepagesInRequestsCheck) Title() string {
	return "Huge Pages Must Not Be In Resource Requests"
}

func (c hugepagesInRequestsCheck) Category() string {
	return "resources"
}

func (c hugepagesInRequestsCheck) Blocking() bool {
	return true
}

func (c hugepagesInRequestsCheck) RenderSensitive() bool {
	return true
}

func (c hugepagesInRequestsCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c hugepagesInRequestsCheck) Run(data []byte, source string) []runtime.Finding {
	info, err := runtime.ExtractPodSpecInfo(data, source)
	if err != nil || info == nil {
		return nil
	}

	var findings []runtime.Finding
	containers := runtime.AllContainers(info)

	for _, ctr := range containers {
		res := ctr.Container.Resources

		if len(res.Requests) == 0 {
			continue
		}

		requests := res.Requests

		for resName, qty := range requests {
			if isHugePageResource(resName) {
				qtyStr := qty.String()
				findings = append(findings, runtime.Finding{
					RuleID:    c.ID(),
					RuleTitle: c.Title(),
					Category:  c.Category(),
					Finding: check.Finding{
						Path:      field.NewPath("spec").Child(info.ContainersPath).Key(ctr.Container.Name).Child("resources").Child("requests").Child(string(resName)).String(),
						Message:   fmt.Sprintf("container %q: %s can only be specified as a limit, not a request", ctr.Container.Name, resName),
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

	return findings
}

// resourceQuantityZeroCheck ensures that resource quantities are not zero.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go:5200-5210
type resourceQuantityZeroCheck struct{}

func (c resourceQuantityZeroCheck) ID() string {
	return "resources/resource-quantity-zero"
}

func (c resourceQuantityZeroCheck) Title() string {
	return "Resource Quantity Must Not Be Zero"
}

func (c resourceQuantityZeroCheck) Category() string {
	return "resources"
}

func (c resourceQuantityZeroCheck) Blocking() bool {
	return true
}

func (c resourceQuantityZeroCheck) RenderSensitive() bool {
	return true
}

func (c resourceQuantityZeroCheck) DocSkipper() []string {
	return runtime.HasPodSpecKinds()
}

func (c resourceQuantityZeroCheck) Run(data []byte, source string) []runtime.Finding {
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
				if qty.IsZero() {
					qtyStr := qty.String()
					findings = append(findings, runtime.Finding{
						RuleID:    c.ID(),
						RuleTitle: c.Title(),
						Category:  c.Category(),
						Finding: check.Finding{
							Path:      path.Child(string(resName)).String(),
							Message:   fmt.Sprintf("container %q: %s: resource quantity must be greater than zero", ctr.Container.Name, resName),
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

// resourceQuantityNegativeCheck ensures that resource quantities are not
// negative. This check catches quantities that compare as less than zero.
// Source: k8s.io/kubernetes/pkg/apis/core/validation/validation.go:5200-5210
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

func (c resourceQuantityNegativeCheck) DocSkipper() []string {
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

// validateResourceQuantity checks whether a resource quantity is valid.
// Returns an empty string if valid, or an error message if not.
func validateResourceQuantity(name corev1.ResourceName, qty resource.Quantity) string {
	// Check for zero quantity
	if qty.IsZero() {
		return "resource quantity must be greater than zero"
	}

	// Check for negative quantity
	if qty.Sign() < 0 {
		return "resource quantity must not be negative"
	}

	// Check for huge pages in requests (handled separately by hugepagesInRequestsCheck)
	if isHugePageResource(name) {
		// Not an error here — huge pages are valid in limits
		return ""
	}

	return ""
}
