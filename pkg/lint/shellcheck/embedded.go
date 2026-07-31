package shellcheck

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// EmbeddedScript is one extracted bash script from a workload container
// command or a ConfigMap .sh/.bash data key.
type EmbeddedScript struct {
	File, ResourceKind, ResourceName, ContainerName string
	Script                                          string
	LineOffset                                      int
}

// EmbeddedResult is one shellcheck run against an extracted embedded
// script.
type EmbeddedResult struct {
	File, ResourceKind, ResourceName, ContainerName string
	Violations                                      []Violation
	Output                                          string
}

// embeddedWorkloadKinds lists the workload kinds whose pod spec (and
// initContainers/containers command fields) are scanned for embedded bash
// scripts.
var embeddedWorkloadKinds = map[string]bool{
	"Pod": true, "Job": true, "CronJob": true, "Deployment": true,
	"DaemonSet": true, "StatefulSet": true, "ReplicaSet": true,
}

// ExtractEmbedded parses a single YAML file (possibly multi-doc) and
// returns every embedded bash script found in a workload's container/
// initContainer command fields, or a ConfigMap's .sh/.bash data keys.
func ExtractEmbedded(path string) ([]EmbeddedScript, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return extractEmbeddedFromBytes(data, path)
}

func extractEmbeddedFromBytes(data []byte, path string) ([]EmbeddedScript, error) {
	var scripts []EmbeddedScript
	dec := yaml.NewDecoder(bytes.NewReader(data))
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
		name := quickName(mapping)
		switch {
		case embeddedWorkloadKinds[kind]:
			scripts = append(scripts, extractWorkloadScripts(mapping, kind, name, path)...)
		case kind == "ConfigMap":
			scripts = append(scripts, extractConfigMapScripts(mapping, name, path)...)
		}
	}
	return scripts, nil
}

// podSpecNode returns the pod spec node for kind. CronJob nests its pod
// spec three levels deeper than every other workload kind
// (spec.jobTemplate.spec.template.spec, not spec.template.spec) - this is
// the same convention already duplicated in pkg/validator/namedport and
// pkg/validator/podspec.
func podSpecNode(mapping *yaml.Node, kind string) *yaml.Node {
	if kind == "CronJob" {
		return getNodeAtPath(mapping, "spec.jobTemplate.spec.template.spec")
	}
	if kind == "Pod" {
		return getNodeAtPath(mapping, "spec")
	}
	return getNodeAtPath(mapping, "spec.template.spec")
}

func extractWorkloadScripts(mapping *yaml.Node, kind, name, path string) []EmbeddedScript {
	podSpec := podSpecNode(mapping, kind)
	if podSpec == nil || podSpec.Kind != yaml.MappingNode {
		return nil
	}
	var scripts []EmbeddedScript
	for _, listKey := range []string{"containers", "initContainers"} {
		list := findKey(podSpec, listKey)
		if list == nil || list.Kind != yaml.SequenceNode {
			continue
		}
		for _, cont := range list.Content {
			if cont.Kind != yaml.MappingNode {
				continue
			}
			cname := quickString(findKey(cont, "name"))
			if s := commandScript(cont); s != nil {
				scripts = append(scripts, EmbeddedScript{
					File: path, ResourceKind: kind, ResourceName: name, ContainerName: cname,
					Script: s.Value, LineOffset: scalarContentLine(s),
				})
			}
		}
	}
	return scripts
}

// commandScript returns the script scalar node from a container's command
// field, if that command is a bash/sh invocation of the shape
// ["<bash-or-sh>", "-c", "<script>"], and the script itself has a bash
// shebang. Non-bash commands (a different interpreter, no "-c" form, no
// command at all) return nil.
func commandScript(container *yaml.Node) *yaml.Node {
	command := findKey(container, "command")
	if command == nil || command.Kind != yaml.SequenceNode || len(command.Content) < 3 {
		return nil
	}
	shell := command.Content[0].Value
	if !strings.Contains(shell, "bash") && !strings.HasSuffix(shell, "/sh") && shell != "sh" {
		return nil
	}
	if command.Content[1].Value != "-c" {
		return nil
	}
	scriptNode := command.Content[2]
	if scriptNode.Kind != yaml.ScalarNode || !isBashScript(scriptNode.Value) {
		return nil
	}
	return scriptNode
}

func extractConfigMapScripts(mapping *yaml.Node, name, path string) []EmbeddedScript {
	data := findKey(mapping, "data")
	if data == nil || data.Kind != yaml.MappingNode {
		return nil
	}
	var scripts []EmbeddedScript
	for i := 0; i < len(data.Content); i += 2 {
		key := data.Content[i]
		val := data.Content[i+1]
		if val.Kind != yaml.ScalarNode {
			continue
		}
		lower := strings.ToLower(key.Value)
		if !strings.HasSuffix(lower, ".sh") && !strings.HasSuffix(lower, ".bash") {
			continue
		}
		if !isBashScript(val.Value) {
			continue
		}
		scripts = append(scripts, EmbeddedScript{
			File: path, ResourceKind: "ConfigMap", ResourceName: name, ContainerName: key.Value,
			Script: val.Value, LineOffset: scalarContentLine(val),
		})
	}
	return scripts
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

// RunEmbedded shellchecks every extracted embedded script across files,
// adjusting shellcheck's temp-file line numbers back to the original
// YAML's line number via LineOffset.
func RunEmbedded(files []string) ([]EmbeddedResult, error) {
	var results []EmbeddedResult
	for _, f := range files {
		if !isYAMLFile(f) {
			continue
		}
		scripts, err := ExtractEmbedded(f)
		if err != nil {
			continue
		}
		for _, es := range scripts {
			violations, output, tmpPath, err := runShellcheckOnScript(es.Script)
			if err != nil {
				if errors.Is(err, ErrCLINotFound) {
					return results, err
				}
				continue
			}
			adjViolations, adjOutput := finalizeScriptResult(violations, output, tmpPath, es.File, es.LineOffset)
			results = append(results, EmbeddedResult{
				File: es.File, ResourceKind: es.ResourceKind, ResourceName: es.ResourceName, ContainerName: es.ContainerName,
				Violations: adjViolations,
				Output:     adjOutput,
			})
		}
	}
	return results, nil
}
