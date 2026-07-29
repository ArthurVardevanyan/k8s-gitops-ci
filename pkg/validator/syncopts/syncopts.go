package syncopts

import (
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

// builtinAPIGroups lists groups considered built-in (no sync-options required).
var builtinAPIGroups = map[string]bool{
	"": true, "apps": true, "batch": true,
	"rbac.authorization.k8s.io": true, "policy": true, "networking.k8s.io": true,
	"storage.k8s.io": true, "scheduling.k8s.io": true, "apiextensions.k8s.io": true,
	"admissionregistration.k8s.io": true, "apiregistration.k8s.io": true,
	"authentication.k8s.io": true, "authorization.k8s.io": true,
	"certificates.k8s.io": true, "coordination.k8s.io": true, "discovery.k8s.io": true,
	"events.k8s.io": true, "flowcontrol.apiserver.k8s.io": true, "node.k8s.io": true,
	"autoscaling": true, "metrics.k8s.io": true, "monitoring.coreos.com": true,
	"operators.coreos.com": true, "operator.openshift.io": true,
	"config.openshift.io": true, "route.openshift.io": true, "image.openshift.io": true,
	"project.openshift.io": true, "quota.openshift.io": true, "security.openshift.io": true,
	"console.openshift.io": true, "helm.openshift.io": true, "tuned.openshift.io": true,
	"machine.openshift.io": true, "machineconfiguration.openshift.io": true,
	"ingressoperator.openshift.io": true, "samples.operator.openshift.io": true,
	"whereabouts.cni.cncf.io": true, "k8s.cni.cncf.io": true,
	"sriovnetwork.openshift.io": true, "nmstate.io": true,
	"hive.openshift.io": true, "agent-install.openshift.io": true,
	"migration.k8s.io": true, "controlplane.operator.openshift.io": true,
	"operatorframework.io": true, "olm.operatorframework.io": true,
	"packages.operators.coreos.com": true, "argoproj.io": false,
	"tekton.dev": false, "kyverno.io": false, "external-secrets.io": false,
	"cert-manager.io": false, "velero.io": false,
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
			if err == io.EOF {
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
	var order []string
	for _, e := range errs {
		key := e.APIVersion + "/" + e.Kind + "/" + e.Name
		if d, ok := seen[key]; ok {
			d.Count++
			continue
		}
		seen[key] = &DeduplicatedError{APIVersion: e.APIVersion, Kind: e.Kind, Name: e.Name, Count: 1}
		order = append(order, key)
	}
	var out []DeduplicatedError
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
	v, ok := builtinAPIGroups[g]
	return ok && v
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
