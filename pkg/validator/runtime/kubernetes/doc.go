// Package kubernetes provides Kubernetes admission-time validation checks.
//
// This package mirrors the validation logic found in k8s.io/kubernetes/pkg/apis/*/validation,
// but implements the rules inline rather than importing the kubernetes repository.
//
// Structure:
//   - core/validation/ — Pod, Container, Volume, Resources, PodSpec validation
//   - apps/validation/ — Deployment, StatefulSet, DaemonSet, ReplicaSet validation
//   - batch/validation/ — Job, CronJob validation
//   - networking/validation/ — NetworkPolicy, Service, Ingress validation
//   - storage/validation/ — PersistentVolume, PersistentVolumeClaim, StorageClass validation
//   - admissionregistration/validation/ — WebhookConfiguration, AdmissionPolicy validation
//   - rbac/validation/ — Role, ClusterRole, RoleBinding validation
//   - policy/validation/ — PodDisruptionBudget validation
package kubernetes
