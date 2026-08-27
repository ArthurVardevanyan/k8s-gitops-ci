package namedport

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ValidationError records a numeric/unnamed port finding.
type ValidationError struct {
	File, Kind, Name, Container, Path, Issue string
	// Value is the stable match target for annotation/selector exemptions
	// (the numeric port, or missing-name port number, the finding is
	// about) - see exempt.Scalar.Value / Scalar.annotationValue().
	Value string
	// Annotations carries the finding's own resource's metadata.annotations,
	// so a gitops-ci.k8s.io/exempt-named-ports annotation on the resource
	// itself can grant an exemption, not just an EXEMPTIONS=(...) selector.
	Annotations map[string]string
}

func (e ValidationError) String() string {
	loc := ""
	if e.Path != "" {
		loc = " at " + e.Path
	}
	if e.Container != "" {
		return fmt.Sprintf("%s: %s %q container %q %s%s", e.File, e.Kind, e.Name, e.Container, e.Issue, loc)
	}
	return fmt.Sprintf("%s: %s %q %s%s", e.File, e.Kind, e.Name, e.Issue, loc)
}

// DeduplicatedError aggregates named-port findings.
type DeduplicatedError struct {
	Kind, Name, Container, Path, Issue string
	Files                              []string
	Count                              int
}

func (d DeduplicatedError) String() string {
	loc := ""
	if d.Path != "" {
		loc = " at " + d.Path
	}
	if d.Container != "" {
		return fmt.Sprintf("%s %q container %q %s%s (%d overlay(s))", d.Kind, d.Name, d.Container, d.Issue, loc, d.Count)
	}
	return fmt.Sprintf("%s %q %s%s (%d overlay(s))", d.Kind, d.Name, d.Issue, loc, d.Count)
}

// ValidateFile validates named ports in a file.
func ValidateFile(path string) []ValidationError {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	return ValidateReader(f, path)
}

// ValidateBytes validates named ports in bytes.
func ValidateBytes(data []byte, source string) []ValidationError {
	return ValidateReader(strings.NewReader(string(data)), source)
}

// ValidateReader validates named ports from a reader.
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
		if kind == "" {
			continue
		}
		name := quickName(mapping)
		annotations := extractAnnotations(findKey(mapping, "metadata"))
		errs = append(errs, validateDoc(mapping, kind, name, source, annotations)...)
	}
	return errs
}

func validateDoc(mapping *yaml.Node, kind, name, source string, annotations map[string]string) []ValidationError {
	var errs []ValidationError
	switch kind {
	case "Deployment", "StatefulSet", "DaemonSet", "Job", "CronJob", "Pod":
		errs = append(errs, validateWorkload(mapping, kind, name, source, annotations)...)
	case "Service":
		errs = append(errs, validateService(mapping, kind, name, source, annotations)...)
	case "Ingress":
		errs = append(errs, validateIngress(mapping, kind, name, source, annotations)...)
	}
	return errs
}

// podSpecPath returns the dot-path (relative to the document root) of the
// pod spec for kind. CronJob nests its pod spec three levels deeper than
// every other workload kind (spec.jobTemplate.spec.template.spec, not
// spec.template.spec) - without this, CronJob workloads are silently never
// validated at all (getNodeAtPath returns nil, and validateWorkload bails
// out with zero findings rather than an error).
func podSpecPath(kind string) string {
	switch kind {
	case "Pod":
		return "spec"
	case "CronJob":
		return "spec.jobTemplate.spec.template.spec"
	default:
		return "spec.template.spec"
	}
}

func validateWorkload(mapping *yaml.Node, kind, name, source string, annotations map[string]string) []ValidationError {
	var errs []ValidationError
	podPath := podSpecPath(kind)
	podSpec := getNodeAtPath(mapping, podPath)
	if podSpec == nil || podSpec.Kind != yaml.MappingNode {
		return nil
	}
	containersPath := podPath + ".containers"
	containers := getNodeAtPath(podSpec, "containers")
	initContainers := getNodeAtPath(podSpec, "initContainers")
	for _, contList := range []*yaml.Node{containers, initContainers} {
		if contList == nil || contList.Kind != yaml.SequenceNode {
			continue
		}
		for _, cont := range contList.Content {
			if cont.Kind != yaml.MappingNode {
				continue
			}
			cname := quickString(findKey(cont, "name"))
			ports := findKey(cont, "ports")
			if ports == nil || ports.Kind != yaml.SequenceNode || len(ports.Content) == 0 {
				// A container with no ports at all is normal, not a
				// violation - only unnamed *existing* ports are flagged
				// below.
				continue
			}
			for i, p := range ports.Content {
				if p.Kind != yaml.MappingNode {
					continue
				}
				portName := quickString(findKey(p, "name"))
				if strings.TrimSpace(portName) == "" {
					containerPort := quickString(findKey(p, "containerPort"))
					errs = append(errs, ValidationError{
						File: source, Kind: kind, Name: name, Container: cname,
						Path:        fmt.Sprintf("%s.ports[%d]", containersPath, i),
						Issue:       fmt.Sprintf("container port %s missing name (define ports[].name so it can be referenced by name)", containerPort),
						Value:       containerPort,
						Annotations: annotations,
					})
				}
			}
			probeTypes := []string{"livenessProbe", "readinessProbe", "startupProbe"}
			for _, pt := range probeTypes {
				probe := findKey(cont, pt)
				if probe == nil || probe.Kind != yaml.MappingNode {
					continue
				}
				httpGet := findKey(probe, "httpGet")
				tcpSocket := findKey(probe, "tcpSocket")
				for _, probeNode := range []*yaml.Node{httpGet, tcpSocket} {
					if probeNode == nil {
						continue
					}
					portField := findKey(probeNode, "port")
					if portField != nil && isNumericPort(portField) {
						errs = append(errs, ValidationError{
							File: source, Kind: kind, Name: name, Container: cname,
							Path:        fmt.Sprintf("%s.%s.port", containersPath, pt),
							Issue:       fmt.Sprintf("%s.port is numeric (%s); reference a named containerPort instead", pt, portField.Value),
							Value:       portField.Value,
							Annotations: annotations,
						})
					}
				}
			}
		}
	}
	return errs
}

func validateService(mapping *yaml.Node, kind, name, source string, annotations map[string]string) []ValidationError {
	var errs []ValidationError
	spec := findKey(mapping, "spec")
	if spec == nil || spec.Kind != yaml.MappingNode {
		return nil
	}
	typeNode := findKey(spec, "type")
	if typeNode != nil && typeNode.Value == "ExternalName" {
		return nil
	}
	ports := findKey(spec, "ports")
	if ports == nil || ports.Kind != yaml.SequenceNode {
		return nil
	}
	for i, p := range ports.Content {
		if p.Kind != yaml.MappingNode {
			continue
		}
		portName := quickString(findKey(p, "name"))
		if strings.TrimSpace(portName) == "" {
			port := quickString(findKey(p, "port"))
			errs = append(errs, ValidationError{
				File: source, Kind: kind, Name: name,
				Path:        fmt.Sprintf("spec.ports[%d]", i),
				Issue:       fmt.Sprintf("service port %s missing name", port),
				Value:       port,
				Annotations: annotations,
			})
		}
		targetPort := findKey(p, "targetPort")
		if targetPort != nil && isNumericPort(targetPort) {
			errs = append(errs, ValidationError{
				File: source, Kind: kind, Name: name,
				Path:        fmt.Sprintf("spec.ports[%d].targetPort", i),
				Issue:       fmt.Sprintf("targetPort is numeric (%s); reference a named containerPort instead", targetPort.Value),
				Value:       targetPort.Value,
				Annotations: annotations,
			})
		}
	}
	return errs
}

func validateIngress(mapping *yaml.Node, kind, name, source string, annotations map[string]string) []ValidationError {
	var errs []ValidationError
	spec := findKey(mapping, "spec")
	if spec == nil || spec.Kind != yaml.MappingNode {
		return nil
	}
	defaultBackend := findKey(spec, "defaultBackend")
	if defaultBackend != nil && defaultBackend.Kind == yaml.MappingNode {
		if svc := findKey(defaultBackend, "service"); svc != nil && svc.Kind == yaml.MappingNode {
			if port := findKey(svc, "port"); port != nil && port.Kind == yaml.MappingNode {
				if n := findKey(port, "number"); n != nil && isNumericPort(n) {
					baseName := quickString(findKey(svc, "name"))
					errs = append(errs, ValidationError{
						File: source, Kind: kind, Name: name,
						Path:        "spec.defaultBackend.service.port.number",
						Issue:       fmt.Sprintf("ingress backend for service %q uses port.number (%s); use port.name instead", baseName, n.Value),
						Value:       n.Value,
						Annotations: annotations,
					})
				}
			}
		}
	}
	rules := findKey(spec, "rules")
	if rules == nil || rules.Kind != yaml.SequenceNode {
		return errs
	}
	for _, rule := range rules.Content {
		http := findKey(rule, "http")
		if http == nil || http.Kind != yaml.MappingNode {
			continue
		}
		paths := findKey(http, "paths")
		if paths == nil || paths.Kind != yaml.SequenceNode {
			continue
		}
		for i, path := range paths.Content {
			if path.Kind != yaml.MappingNode {
				continue
			}
			backend := findKey(path, "backend")
			if backend == nil || backend.Kind != yaml.MappingNode {
				continue
			}
			svc := findKey(backend, "service")
			if svc == nil || svc.Kind != yaml.MappingNode {
				continue
			}
			port := findKey(svc, "port")
			if port == nil || port.Kind != yaml.MappingNode {
				continue
			}
			if n := findKey(port, "number"); n != nil && isNumericPort(n) {
				baseName := quickString(findKey(svc, "name"))
				errs = append(errs, ValidationError{
					File: source, Kind: kind, Name: name,
					Path:        fmt.Sprintf("spec.rules[].http.paths[%d].backend.service.port.number", i),
					Issue:       fmt.Sprintf("ingress backend for service %q uses port.number (%s); use port.name instead", baseName, n.Value),
					Value:       n.Value,
					Annotations: annotations,
				})
			}
		}
	}
	return errs
}

func isNumericPort(n *yaml.Node) bool {
	if n == nil || n.Kind != yaml.ScalarNode {
		return false
	}
	_, err := strconv.Atoi(strings.TrimSpace(n.Value))
	return err == nil
}

// Deduplicate aggregates findings, capping sample files.
func Deduplicate(errors []ValidationError, maxFiles int) []DeduplicatedError {
	if maxFiles <= 0 {
		maxFiles = 3
	}
	seen := make(map[string]*DeduplicatedError)
	order := make([]string, 0, len(errors))
	for _, e := range errors {
		key := e.Kind + "/" + e.Name + "/" + e.Container + "/" + e.Path + "/" + e.Issue
		if d, ok := seen[key]; ok {
			d.Count++
			if len(d.Files) < maxFiles && !contains(d.Files, e.File) {
				d.Files = append(d.Files, e.File)
			}
			continue
		}
		files := []string{e.File}
		seen[key] = &DeduplicatedError{
			Kind: e.Kind, Name: e.Name, Container: e.Container, Path: e.Path,
			Issue: e.Issue, Files: files, Count: 1,
		}
		order = append(order, key)
	}
	out := make([]DeduplicatedError, 0, len(order))
	for _, k := range order {
		out = append(out, *seen[k])
	}
	return out
}

func getNodeAtPath(root *yaml.Node, path string) *yaml.Node {
	parts := strings.Split(path, ".")
	cur := root
	for _, p := range parts {
		if cur == nil || cur.Kind != yaml.MappingNode {
			return nil
		}
		cur = findKey(cur, p)
	}
	return cur
}

// extractAnnotations returns a resource's metadata.annotations, so callers
// can plumb them onto a Finding for annotation-based exemptions
// (gitops-ci.k8s.io/exempt-named-ports).
func extractAnnotations(meta *yaml.Node) map[string]string {
	out := make(map[string]string)
	if meta == nil || meta.Kind != yaml.MappingNode {
		return out
	}
	obj := findKey(meta, "annotations")
	if obj == nil || obj.Kind != yaml.MappingNode {
		return out
	}
	for i := 0; i < len(obj.Content); i += 2 {
		out[obj.Content[i].Value] = obj.Content[i+1].Value
	}
	return out
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

func contains(sl []string, s string) bool {
	for _, x := range sl {
		if x == s {
			return true
		}
	}
	return false
}
