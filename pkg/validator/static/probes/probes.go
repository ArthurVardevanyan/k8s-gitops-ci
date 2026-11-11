package probes

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// WorkloadKinds lists kinds that contain a pod template.
var WorkloadKinds = map[string]bool{
	"Deployment": true, "StatefulSet": true, "DaemonSet": true,
	"Job": true, "CronJob": true, "ReplicaSet": true, "Pod": true,
}

// ValidationError records a container missing a probe.
type ValidationError struct {
	File, Kind, Name, Container string
	Path                        string
	Annotations                 map[string]string
	Probe                       string // "liveness" or "readiness"
}

func (e ValidationError) String() string {
	return fmt.Sprintf("%s: %s %q container %q: missing %s probe", e.File, e.Kind, e.Name, e.Container, e.Probe)
}

// ValidateLivenessProbes validates liveness probes in a file, checking both
// regular and init containers (init containers can be restarted by a failing
// liveness probe, so they are in scope).
func ValidateLivenessProbes(data []byte, source string) []ValidationError {
	return validateProbes(ValidateOptions{
		Data:    data,
		Source:  source,
		Field:   "livenessProbe",
		Probe:   "liveness",
		InitToo: true,
	})
}

// ValidateReadinessProbes validates readiness probes in a file. Init
// containers are skipped: they never receive traffic, so a readiness probe
// on them is meaningless.
func ValidateReadinessProbes(data []byte, source string) []ValidationError {
	return validateProbes(ValidateOptions{
		Data:    data,
		Source:  source,
		Field:   "readinessProbe",
		Probe:   "readiness",
		InitToo: false,
	})
}

// ValidateFile validates liveness probes in a file.
func ValidateFile(path string) []ValidationError {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return ValidateLivenessProbes(data, path)
}

// ValidateBytes validates liveness probes in bytes. Retained for callers that
// predate the liveness/readiness split.
func ValidateBytes(data []byte, source string) []ValidationError {
	return ValidateLivenessProbes(data, source)
}

// ValidateOptions configures validateProbes.
type ValidateOptions struct {
	Data    []byte
	Source  string
	Field   string // YAML field to look for, e.g. "livenessProbe"
	Probe   string // human-readable probe name, e.g. "liveness"
	InitToo bool   // whether to also check initContainers
}

func validateProbes(opts ValidateOptions) []ValidationError {
	var errs []ValidationError
	dec := yaml.NewDecoder(strings.NewReader(string(opts.Data)))
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
		if !WorkloadKinds[kind] {
			continue
		}
		name := quickName(mapping)
		podPath := podSpecPath(kind)
		podSpec := getNodeAtPath(mapping, podPath)
		if podSpec == nil || podSpec.Kind != yaml.MappingNode {
			continue
		}
		fields := []string{"containers"}
		if opts.InitToo {
			fields = append(fields, "initContainers")
		}
		for _, contField := range fields {
			contList := getNodeAtPath(podSpec, contField)
			if contList == nil || contList.Kind != yaml.SequenceNode {
				continue
			}
			for _, cont := range contList.Content {
				if cont.Kind != yaml.MappingNode {
					continue
				}
				cname := quickString(findKey(cont, "name"))
				if !hasProbe(cont, opts.Field) {
					errs = append(errs, ValidationError{
						File: opts.Source, Kind: kind, Name: name, Container: cname,
						Path:  fmt.Sprintf("%s.%s[].%s", podPath, contField, opts.Field),
						Probe: opts.Probe,
					})
				}
			}
		}
	}
	return errs
}

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

func hasProbe(container *yaml.Node, field string) bool {
	return findKey(container, field) != nil
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
