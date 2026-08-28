package runtime

import (
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"
)

// ContainerWithPath holds a container with its path context.
type ContainerWithPath struct {
	Container corev1.Container
	Path      *field.Path
	IsInit    bool
}

// AllContainers builds the full container list (regular + init) with path context.
func AllContainers(info *PodSpecInfo) []ContainerWithPath {
	out := make([]ContainerWithPath, 0, len(info.Containers)+len(info.InitContainers))
	for i := range info.Containers {
		out = append(out, ContainerWithPath{
			Container: info.Containers[i],
			Path:      field.NewPath(info.ContainersPath).Key(info.Containers[i].Name),
			IsInit:    false,
		})
	}
	for i := range info.InitContainers {
		out = append(out, ContainerWithPath{
			Container: info.InitContainers[i],
			Path:      field.NewPath(info.InitContainersPath).Key(info.InitContainers[i].Name),
			IsInit:    true,
		})
	}
	return out
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
	// PodSpecPath is the dotted path to the PodSpec within the document
	// ("spec" for a Pod, "spec.template.spec" for a controller,
	// "spec.jobTemplate.spec.template.spec" for a CronJob).
	//
	// Checks that report a field inside the pod spec must build their path
	// from this rather than hard-coding "spec.*": a finding on a Deployment
	// that points at spec.volumes sends the reader to a field that does not
	// exist in their manifest. ContainersPath and InitContainersPath are
	// derived from it so the three cannot drift apart.
	PodSpecPath        string
	ContainersPath     string
	InitContainersPath string
}

// VolumesPath returns the dotted path to the pod spec's volumes list for the
// workload kind this info was extracted from.
func (i *PodSpecInfo) VolumesPath() string {
	return i.PodSpecPath + ".volumes"
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
		podSpec, err = extractPodSpecAt(&unstructuredObj, "spec")
		info.PodSpecPath = "spec"
	case "CronJob":
		podSpec, err = extractPodSpecAt(&unstructuredObj, "spec", "jobTemplate", "spec", "template", "spec")
		info.PodSpecPath = "spec.jobTemplate.spec.template.spec"
	default:
		podSpec, err = extractPodSpecAt(&unstructuredObj, "spec", "template", "spec")
		info.PodSpecPath = "spec.template.spec"
	}
	info.ContainersPath = info.PodSpecPath + ".containers"
	info.InitContainersPath = info.PodSpecPath + ".initContainers"

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

// extractPodSpecAt decodes the PodSpec at the given path within obj. It
// returns nil, nil when the path is absent or empty, meaning there is no pod
// spec to validate.
func extractPodSpecAt(obj *unstructured.Unstructured, path ...string) (*corev1.PodSpec, error) {
	spec, found, err := unstructured.NestedMap(obj.UnstructuredContent(), path...)
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
