package runtime

import (
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"
)

// ContainerWithPath holds a container with its path context.
type ContainerWithPath struct {
	Container corev1.Container
	Path      *field.Path
	IsInit    bool
}

// PodSpecInfo holds extracted pod spec information for validation.
type PodSpecInfo struct {
	Kind               string
	Name               string
	Namespace          string
	APIVersion         string
	Source             string
	PodSpec            corev1.PodSpec
	PodSecurityContext *corev1.PodSecurityContext
	Containers         []corev1.Container
	InitContainers     []corev1.Container
	ContainersPath     string
	InitContainersPath string
}

// ExtractPodSpecInfo parses a YAML document and extracts pod spec information.
// Returns nil if the document is not a workload kind with a pod spec.
func ExtractPodSpecInfo(data []byte, source string) (*PodSpecInfo, error) {
	var unstructuredObj unstructured.Unstructured
	if err := yaml.Unmarshal(data, &unstructuredObj); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	kind := unstructuredObj.GetKind()
	if !IsPodSpecKind(kind) {
		return nil, nil
	}

	info := &PodSpecInfo{
		Kind:       kind,
		Name:       unstructuredObj.GetName(),
		Namespace:  unstructuredObj.GetNamespace(),
		APIVersion: unstructuredObj.GetAPIVersion(),
		Source:     source,
	}

	var podSpec *corev1.PodSpec
	var err error

	switch kind {
	case "Pod":
		podSpec, err = extractPodSpecFromUnstructured(&unstructuredObj)
		info.ContainersPath = "spec.containers"
		info.InitContainersPath = "spec.initContainers"
	case "CronJob":
		podSpec, err = extractPodSpecFromCronJob(&unstructuredObj)
		info.ContainersPath = "spec.jobTemplate.spec.template.spec.containers"
		info.InitContainersPath = "spec.jobTemplate.spec.template.spec.initContainers"
	default:
		podSpec, err = extractPodSpecFromTemplate(&unstructuredObj)
		info.ContainersPath = "spec.template.spec.containers"
		info.InitContainersPath = "spec.template.spec.initContainers"
	}

	if err != nil {
		return nil, fmt.Errorf("failed to extract pod spec for %s %s: %w", kind, info.Name, err)
	}

	if podSpec == nil {
		return nil, nil
	}

	info.PodSpec = *podSpec
	info.Containers = podSpec.Containers
	info.InitContainers = podSpec.InitContainers
	info.PodSecurityContext = podSpec.SecurityContext

	return info, nil
}

func extractPodSpecFromUnstructured(obj *unstructured.Unstructured) (*corev1.PodSpec, error) {
	podData, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&corev1.Pod{Spec: corev1.PodSpec{}})
	if err != nil {
		return nil, err
	}

	for k, v := range obj.UnstructuredContent() {
		if k == "spec" {
			podData["spec"] = v
		}
	}

	podBytes, err := json.Marshal(podData)
	if err != nil {
		return nil, err
	}

	var pod corev1.Pod
	if err := yaml.Unmarshal(podBytes, &pod); err != nil {
		return nil, err
	}

	return &pod.Spec, nil
}

func extractPodSpecFromTemplate(obj *unstructured.Unstructured) (*corev1.PodSpec, error) {
	spec, found, err := unstructured.NestedMap(obj.UnstructuredContent(), "spec", "template", "spec")
	if err != nil {
		return nil, err
	}
	if !found || len(spec) == 0 {
		return nil, nil
	}

	specJSON, err := json.Marshal(spec)
	if err != nil {
		return nil, err
	}

	var podSpec corev1.PodSpec
	if err := yaml.Unmarshal(specJSON, &podSpec); err != nil {
		return nil, err
	}

	return &podSpec, nil
}

func extractPodSpecFromCronJob(obj *unstructured.Unstructured) (*corev1.PodSpec, error) {
	spec, found, err := unstructured.NestedMap(obj.UnstructuredContent(), "spec", "jobTemplate", "spec", "template", "spec")
	if err != nil {
		return nil, err
	}
	if !found || len(spec) == 0 {
		return nil, nil
	}

	specJSON, err := json.Marshal(spec)
	if err != nil {
		return nil, err
	}

	var podSpec corev1.PodSpec
	if err := yaml.Unmarshal(specJSON, &podSpec); err != nil {
		return nil, err
	}

	return &podSpec, nil
}

// HasPodSpecKinds returns the Kubernetes kinds that have a PodSpec.
func HasPodSpecKinds() []string {
	return []string{
		"Pod",
		"Deployment",
		"StatefulSet",
		"DaemonSet",
		"ReplicaSet",
		"Job",
		"CronJob",
		"ReplicationController",
	}
}

// IsPodSpecKind returns true if the given kind has a PodSpec.
func IsPodSpecKind(kind string) bool {
	for _, k := range HasPodSpecKinds() {
		if k == kind {
			return true
		}
	}
	return false
}
