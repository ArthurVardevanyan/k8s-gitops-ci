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
	// IsEphemeral marks a container from spec.ephemeralContainers. Upstream
	// validates these with the same validateContainerCommon as the other
	// two lists and requires their names to be unique across all three.
	IsEphemeral bool
}

// AllContainers builds the full container list (regular + init + ephemeral)
// with path context.
//
// All three lists are included because upstream validates all three: a rule
// applied only to spec.containers silently exempts whatever is declared in
// the other lists, which is the opposite of what a non-exemptable family
// should do.
func AllContainers(info *PodSpecInfo) []ContainerWithPath {
	out := make([]ContainerWithPath, 0,
		len(info.Containers)+len(info.InitContainers)+len(info.EphemeralContainers))
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
	for i := range info.EphemeralContainers {
		out = append(out, ContainerWithPath{
			Container:   info.EphemeralContainers[i],
			Path:        field.NewPath(info.EphemeralContainersPath).Key(info.EphemeralContainers[i].Name),
			IsEphemeral: true,
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
	// EphemeralContainers holds spec.ephemeralContainers as plain
	// Containers. The upstream type embeds EphemeralContainerCommon, which
	// is field-for-field the same as Container apart from the fields
	// ephemeral containers may not set, so the shared rules apply unchanged.
	EphemeralContainers []corev1.Container
	// PodSpecPath is the dotted path to the PodSpec within the document
	// ("spec" for a Pod, "spec.template.spec" for a controller,
	// "spec.jobTemplate.spec.template.spec" for a CronJob).
	//
	// Checks that report a field inside the pod spec must build their path
	// from this rather than hard-coding "spec.*": a finding on a Deployment
	// that points at spec.volumes sends the reader to a field that does not
	// exist in their manifest. ContainersPath and InitContainersPath are
	// derived from it so the three cannot drift apart.
	PodSpecPath             string
	ContainersPath          string
	InitContainersPath      string
	EphemeralContainersPath string
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
	info.EphemeralContainersPath = info.PodSpecPath + ".ephemeralContainers"

	if err != nil {
		return nil, fmt.Errorf("failed to extract pod spec for %s %s: %w", kind, info.Name, err)
	}

	if podSpec == nil {
		return nil, nil
	}

	info.PodSpec = *podSpec
	info.Containers = podSpec.Containers
	info.InitContainers = podSpec.InitContainers
	for i := range podSpec.EphemeralContainers {
		info.EphemeralContainers = append(info.EphemeralContainers,
			corev1.Container(podSpec.EphemeralContainers[i].EphemeralContainerCommon))
	}
	info.PodSecurityContext = podSpec.SecurityContext

	if kind == "StatefulSet" {
		addVolumeClaimTemplateVolumes(&unstructuredObj, info)
	}

	return info, nil
}

// addVolumeClaimTemplateVolumes injects a StatefulSet's volumeClaimTemplates
// into the pod spec's volume list as the API server does.
//
// A StatefulSet's containers mount their persistent storage by the name of a
// volumeClaimTemplate, not of a volume declared in the pod template - that is
// the whole point of the field, and nearly every real StatefulSet is written
// that way. Upstream validates the template only after synthesizing a volume
// for each claim template (ValidateStatefulSetSpec -> volumesToAddForTemplates),
// so a mount naming one resolves and is accepted.
//
// Porting the mount rule without this step reports every such StatefulSet as
// mounting an undefined volume - a false positive on a non-exemptable check,
// against the most common way the kind is used.
//
// The claim templates are added first and a pod-template volume of the same
// name is dropped, matching upstream's precedence, so that a name appearing in
// both does not read as a duplicate volume.
func addVolumeClaimTemplateVolumes(obj *unstructured.Unstructured, info *PodSpecInfo) {
	templates, found, err := unstructured.NestedSlice(obj.UnstructuredContent(), "spec", "volumeClaimTemplates")
	if err != nil || !found || len(templates) == 0 {
		return
	}

	claimed := make(map[string]bool, len(templates))
	merged := make([]corev1.Volume, 0, len(templates)+len(info.PodSpec.Volumes))
	for _, t := range templates {
		tm, ok := t.(map[string]interface{})
		if !ok {
			continue
		}
		name, found, err := unstructured.NestedString(tm, "metadata", "name")
		if err != nil || !found || name == "" {
			continue
		}
		// Collapse duplicate template names, as upstream does. Its
		// volumesToAddForTemplates builds a map[string]api.Volume keyed by
		// name, so two templates sharing a name contribute one synthetic
		// volume; validateVolumeClaimTemplates does not reject the duplicate
		// name either. Appending both here synthesized a name collision that
		// upstream never sees, and volume/duplicate-volume-names then
		// reported it - a finding no manifest change could satisfy and no
		// exemption could suppress.
		//
		// Which duplicate survives is immaterial: both synthetic volumes are
		// built from the same name, so they are identical. Keeping the first
		// is deterministic, where upstream's map iteration is not.
		if claimed[name] {
			continue
		}
		claimed[name] = true
		merged = append(merged, corev1.Volume{
			Name: name,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: name},
			},
		})
	}
	if len(merged) == 0 {
		return
	}
	for _, v := range info.PodSpec.Volumes {
		if !claimed[v.Name] {
			merged = append(merged, v)
		}
	}
	info.PodSpec.Volumes = merged
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
