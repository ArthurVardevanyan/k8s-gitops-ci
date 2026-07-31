package syncopts

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	RequiredAnnotation = "argocd.argoproj.io/sync-options"
	RequiredValue      = "SkipDryRunOnMissingResource=true"
	Marker             = "<!-- sync-options-warning -->"
)

// AssumeOpenShift enables treating OpenShift/OKD-only API groups (OLM,
// Prometheus Operator, *.openshift.io, SR-IOV/Multus CNI, Gateway API,
// the built-in image registry, Metal3, etc.) as builtin/exempt from the
// sync-options requirement. These groups only ship by default on
// OpenShift/OKD clusters — enable this only if ALL target clusters are
// OpenShift/OKD. Set once at process startup (see validator.RunAll).
var AssumeOpenShift = false

// coreAPIGroups lists distro-agnostic API groups considered "built-in" (no
// sync-options annotation required) on any conformant Kubernetes cluster.
var coreAPIGroups = map[string]bool{
	// Core / built-in Kubernetes API groups.
	"":                             true, // core/v1
	"apps":                         true,
	"batch":                        true,
	"autoscaling":                  true,
	"rbac.authorization.k8s.io":    true,
	"policy":                       true,
	"networking.k8s.io":            true,
	"storage.k8s.io":               true,
	"scheduling.k8s.io":            true,
	"apiextensions.k8s.io":         true,
	"admissionregistration.k8s.io": true,
	"apiregistration.k8s.io":       true,
	"authentication.k8s.io":        true,
	"authorization.k8s.io":         true,
	"certificates.k8s.io":          true,
	"coordination.k8s.io":          true,
	"discovery.k8s.io":             true,
	"events.k8s.io":                true,
	"flowcontrol.apiserver.k8s.io": true,
	"node.k8s.io":                  true,
	"migration.k8s.io":             true,
	"policy.networking.k8s.io":     true, // upstream AdminNetworkPolicy
	"populator.storage.k8s.io":     true, // upstream volume-populator API group
	"snapshot.storage.k8s.io":      true, // upstream CSI snapshot API group
	"internal.apiserver.k8s.io":    true, // built-in apiserver-internal group
	"resource.k8s.io":              true, // upstream DRA API group
	"storagemigration.k8s.io":      true, // built-in storage-migration API group

	// Kustomize build-time control objects (Kustomization/Component).
	// Never applied to the cluster by ArgoCD; the annotation concept
	// doesn't apply to them.
	"kustomize.config.k8s.io": true,

	// Widely-installed, distro-agnostic platform group.
	"metrics.k8s.io": true,

	// Kyverno CLI-only local test fixture (Test kind) - never applied to
	// a cluster, analogous to installerOnlyKinds, not distro-specific.
	"cli.kyverno.io": true,

	// Upstream Cluster API groups, not OpenShift-exclusive.
	"infrastructure.cluster.x-k8s.io": true,
	"ipam.cluster.x-k8s.io":           true,
}

// openshiftAPIGroups lists API groups that ship by default on OpenShift/OKD
// clusters (but not on a generic/vanilla Kubernetes cluster). These are only
// treated as exempt when AssumeOpenShift is true.
var openshiftAPIGroups = map[string]bool{
	// Prometheus Operator / OLM — bundled with OpenShift/OKD's cluster
	// monitoring and Operator Lifecycle Manager respectively.
	"monitoring.coreos.com": true,
	"operators.coreos.com":  true,

	// OpenShift API groups.
	"operator.openshift.io":               true,
	"config.openshift.io":                 true,
	"route.openshift.io":                  true,
	"image.openshift.io":                  true,
	"imageregistry.operator.openshift.io": true,
	"project.openshift.io":                true,
	"quota.openshift.io":                  true,
	"security.openshift.io":               true,
	"console.openshift.io":                true,
	"helm.openshift.io":                   true,
	"tuned.openshift.io":                  true,
	"machine.openshift.io":                true,
	"machineconfiguration.openshift.io":   true,
	"ingress.operator.openshift.io":       true,
	"samples.operator.openshift.io":       true,
	"hive.openshift.io":                   true,
	"agent-install.openshift.io":          true,
	"controlplane.operator.openshift.io":  true,

	// Gateway API — OpenShift/OKD's built-in Gateway API implementation
	// (Cluster Ingress Operator), not shipped by default on vanilla k8s.
	"gateway.networking.k8s.io": true,

	// Cluster Baremetal Operator — ships on OpenShift/OKD baremetal-
	// platform installs.
	"metal3.io": true,

	// CNI / networking-related operator groups.
	"whereabouts.cni.cncf.io":   true,
	"k8s.cni.cncf.io":           true,
	"sriovnetwork.openshift.io": true,
	"nmstate.io":                true,

	// OLM / operator framework groups.
	"operatorframework.io":          true,
	"olm.operatorframework.io":      true,
	"packages.operators.coreos.com": true,

	// Ships with OLM's default catalog on OpenShift/OKD.
	"operatorhub.io": true,

	// CNI, grouped with the existing whereabouts/k8s.cni.cncf.io entries
	// above for consistency.
	"k8s.ovn.org": true,

	// OpenShift-namespaced groups.
	"network.openshift.io":           true,
	"network.operator.openshift.io":  true,
	"cloud.network.openshift.io":     true,
	"build.openshift.io":             true,
	"apps.openshift.io":              true,
	"template.openshift.io":          true,
	"authorization.openshift.io":     true,
	"user.openshift.io":              true,
	"oauth.openshift.io":             true,
	"security.internal.openshift.io": true,
	"monitoring.openshift.io":        true,
	"cloudcredential.openshift.io":   true,
	"performance.openshift.io":       true,
	"apiserver.openshift.io":         true,
	"autoscaling.openshift.io":       true,
}

// nonExemptCRDGroups lists known CRD-providing groups that are NOT exempt —
// resources in these groups still require the sync-options annotation,
// since they're optional add-ons (installed via OLM/Helm) on any platform,
// including OpenShift/OKD, and may not exist yet at first sync.
var nonExemptCRDGroups = map[string]bool{
	"argoproj.io":         false,
	"tekton.dev":          false,
	"kyverno.io":          false,
	"external-secrets.io": false,
	"cert-manager.io":     false,
	"velero.io":           false,
	"networking.istio.io": false,
	"security.istio.io":   false,
}

// installerOnlyKinds are local config artifacts consumed by installer
// tooling (e.g. the OpenShift agent-based installer) that are never
// submitted to a Kubernetes API server or synced by ArgoCD. They're
// typically declared with a bare, groupless apiVersion (e.g. "v1beta1"),
// so they're matched by kind instead of by API group.
var installerOnlyKinds = map[string]bool{
	"AgentConfig":   true,
	"InstallConfig": true,
}

// ValidationError records a missing sync-options annotation.
type ValidationError struct {
	File, Name, Kind, APIVersion string
}

func (e ValidationError) String() string {
	return fmt.Sprintf("%s: %s/%s %q is missing annotation %s: %s", e.File, e.APIVersion, e.Kind, e.Name, RequiredAnnotation, RequiredValue)
}

// DeduplicatedError aggregates sync-options violations.
type DeduplicatedError struct {
	APIVersion, Kind, Name string
	Count                  int
}

func (d DeduplicatedError) String() string {
	return fmt.Sprintf("%s/%s %q missing %s (%d overlay(s))", d.APIVersion, d.Kind, d.Name, RequiredAnnotation, d.Count)
}

// ValidateFile validates sync-options in a file.
func ValidateFile(path string) []ValidationError {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	return ValidateReader(f, path)
}

// ValidateReader validates sync-options from a reader.
func ValidateReader(r io.Reader, source string) []ValidationError {
	var errs []ValidationError
	dec := yaml.NewDecoder(r)
	for {
		var doc yaml.Node
		if err := dec.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			break
		}
		if len(doc.Content) == 0 {
			continue
		}
		mapping := doc.Content[0]
		if mapping.Kind != yaml.MappingNode {
			continue
		}
		kind := quickString(findKey(mapping, "kind"))
		apiVersion := quickString(findKey(mapping, "apiVersion"))
		if kind == "" || apiVersion == "" {
			continue
		}
		if installerOnlyKinds[kind] {
			continue
		}
		if isBuiltinResource(apiVersion) {
			continue
		}
		if hasSkipDryRun(extractAnnotations(mapping)) {
			continue
		}
		name := quickName(mapping)
		errs = append(errs, ValidationError{File: source, Kind: kind, APIVersion: apiVersion, Name: name})
	}
	return errs
}

// Deduplicate aggregates sync-options errors.
func Deduplicate(errs []ValidationError) []DeduplicatedError {
	seen := make(map[string]*DeduplicatedError)
	order := make([]string, 0, len(errs))
	for _, e := range errs {
		key := e.APIVersion + "/" + e.Kind + "/" + e.Name
		if d, ok := seen[key]; ok {
			d.Count++
			continue
		}
		seen[key] = &DeduplicatedError{APIVersion: e.APIVersion, Kind: e.Kind, Name: e.Name, Count: 1}
		order = append(order, key)
	}
	out := make([]DeduplicatedError, 0, len(order))
	for _, k := range order {
		out = append(out, *seen[k])
	}
	return out
}

// FormatComment renders sync-options findings.
func FormatComment(deduped []DeduplicatedError) string {
	if len(deduped) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(Marker + "\n")
	b.WriteString("### Missing Sync Options Annotation\n\n")
	b.WriteString("| Resource | Overlays |\n")
	b.WriteString("| --- | --- |\n")
	for _, d := range deduped {
		fmt.Fprintf(&b, "| `%s/%s` %q | %d |\n", d.APIVersion, d.Kind, d.Name, d.Count)
	}
	b.WriteString("\n<details><summary>Required annotation</summary>\n\n")
	fmt.Fprintf(&b, "```yaml\nmetadata:\n  annotations:\n    %s: %s\n```\n", RequiredAnnotation, RequiredValue)
	b.WriteString("</details>\n")
	return b.String()
}

func extractGroup(apiVersion string) string {
	if apiVersion == "v1" {
		return ""
	}
	if idx := strings.Index(apiVersion, "/"); idx != -1 {
		return apiVersion[:idx]
	}
	return apiVersion
}

func isBuiltinResource(apiVersion string) bool {
	g := extractGroup(apiVersion)
	if v, ok := coreAPIGroups[g]; ok {
		return v
	}
	if AssumeOpenShift {
		if v, ok := openshiftAPIGroups[g]; ok {
			return v
		}
	}
	if v, ok := nonExemptCRDGroups[g]; ok {
		return v
	}
	return false
}

func hasSkipDryRun(annotations map[string]string) bool {
	for k, v := range annotations {
		if k == RequiredAnnotation && strings.Contains(v, RequiredValue) {
			return true
		}
	}
	return false
}

func extractAnnotations(mapping *yaml.Node) map[string]string {
	ann := make(map[string]string)
	if meta := findKey(mapping, "metadata"); meta != nil && meta.Kind == yaml.MappingNode {
		obj := findKey(meta, "annotations")
		if obj == nil || obj.Kind != yaml.MappingNode {
			return ann
		}
		for i := 0; i < len(obj.Content); i += 2 {
			ann[obj.Content[i].Value] = obj.Content[i+1].Value
		}
	}
	return ann
}

func quickName(mapping *yaml.Node) string {
	if meta := findKey(mapping, "metadata"); meta != nil && meta.Kind == yaml.MappingNode {
		if n := quickString(findKey(meta, "name")); n != "" {
			return n
		}
	}
	return "(unnamed)"
}

func quickString(n *yaml.Node) string {
	if n == nil {
		return ""
	}
	return n.Value
}

func findKey(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}
