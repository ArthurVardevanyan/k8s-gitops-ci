package livenessprobe

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

// ValidationError records a container missing a liveness probe.
type ValidationError struct {
	File, Kind, Name, Container string
	Path                        string
	Annotations                 map[string]string
}

func (e ValidationError) String() string {
	return fmt.Sprintf("%s: %s %q container %q: missing liveness probe", e.File, e.Kind, e.Name, e.Container)
}

// ValidateFile validates liveness probes in a file.
func ValidateFile(path string) []ValidationError {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	return ValidateReader(f, path)
}

// ValidateBytes validates liveness probes in bytes.
func ValidateBytes(data []byte, source string) []ValidationError {
	return ValidateReader(strings.NewReader(string(data)), source)
}

// ValidateReader validates liveness probes from a reader.
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
		if !WorkloadKinds[kind] {
			continue
		}
		name := quickName(mapping)
		podPath := podSpecPath(kind)
		podSpec := getNodeAtPath(mapping, podPath)
		if podSpec == nil || podSpec.Kind != yaml.MappingNode {
			continue
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
				if !hasLivenessProbe(cont) {
					errs = append(errs, ValidationError{
						File: source, Kind: kind, Name: name, Container: cname,
						Path: fmt.Sprintf("%s[].livenessProbe", containersPath),
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

func hasLivenessProbe(container *yaml.Node) bool {
	probe := findKey(container, "livenessProbe")
	return probe != nil
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
