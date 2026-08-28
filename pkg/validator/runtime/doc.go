// Package runtime provides runtime validation checks that catch errors
// Kubernetes enforces at admission time but cannot be expressed in OpenAPI schemas.
//
// Checks are organized in pkg/validator/runtime/kubernetes/*.go, following
// the categories found in k8s.io/kubernetes/pkg/apis/core/validation:
//
//   - security_context.go — combinatorial security context constraints
//   - container.go — container-level validations (ports, images, names, probes)
//   - volume.go — volume and mount validations
//   - resources.go — resource limits/requests and QoS validations
//   - pod_spec.go — pod spec level validations (affinity, tolerations, etc.)
//
// All checks use k8s.io/api/core/v1 typed structs for parsing YAML,
// with validation rules rewritten inline from k8s.io/kubernetes.
package runtime
