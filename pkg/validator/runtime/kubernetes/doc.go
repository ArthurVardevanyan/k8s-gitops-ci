// Package kubernetes registers the runtime validation checks, grouped by the
// upstream API group whose validation they port.
//
// Each subpackage mirrors the rules in k8s.io/kubernetes/pkg/apis/<group>/
// validation, reimplemented against the k8s.io/api typed structs rather than
// importing k8s.io/kubernetes. Every check cites the upstream function it
// ports through its package's upstreamRefs table; see the runtime package
// documentation and docs/CI.md.
//
// Subpackages:
//   - core/validation — containers, pod spec, volumes, resources, ConfigMap,
//     LimitRange, ResourceQuota, and object metadata (name and namespace).
//   - apps/validation — Deployment, StatefulSet, DaemonSet, ReplicaSet.
//   - batch/validation — Job, CronJob.
//   - autoscaling/validation — HorizontalPodAutoscaler.
//   - networking/validation — Service, Ingress, NetworkPolicy.
//   - storage/validation — PersistentVolume, PersistentVolumeClaim,
//     StorageClass.
//   - policy/validation — PodDisruptionBudget.
//   - rbac/validation — RoleBinding and ClusterRoleBinding. Role and
//     ClusterRole are not validated here.
//   - admissionregistration/validation — ValidatingWebhookConfiguration and
//     MutatingWebhookConfiguration. The policy types (ValidatingAdmissionPolicy
//     and friends) are not ported.
//   - apiextensions/validation — CustomResourceDefinition.
//
// Each subpackage registers its checks from an init() so that a blank import
// here is enough to install them; register.go is the single place that lists
// those imports.
package kubernetes
