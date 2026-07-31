package shellcheck

import (
	"bytes"
	"errors"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// TektonScript is one extracted Tekton Task step script.
type TektonScript struct {
	File, TaskName, StepName string
	Script                   string
	// LineOffset is the line number in File where Script begins (i.e. the
	// first line of the block scalar's content, not the "script:" key
	// line itself).
	LineOffset int
}

// TektonResult is one shellcheck run against an extracted Tekton step
// script.
type TektonResult struct {
	File, TaskName, StepName string
	Violations               []Violation
	Output                   string
}

// bashShebangRe matches the shebang lines this package treats as "bash" for
// the purposes of embedded-script extraction: "#!/bin/bash",
// "#!/usr/bin/bash", and "#!/usr/bin/env bash" (with optional trailing
// arguments, e.g. "#!/usr/bin/env bash -eu"). Anything else (python, plain
// sh, no shebang at all) is not linted here - shellcheck only understands
// bash/sh/dash/ksh, and treating an arbitrary interpreter's source as shell
// would produce meaningless findings.
var bashShebangRe = regexp.MustCompile(`(?m)^#!\s*(?:/usr/bin/env\s+bash|/(?:usr/)?bin/bash)\b`)

// isBashScript classifies a script's shebang as bash (true) vs. anything
// else (python/sh/no-shebang -> false).
func isBashScript(script string) bool {
	return bashShebangRe.MatchString(script)
}

// FilterTektonTasks returns files that are (or plausibly are) Tekton Task
// manifests: any YAML file, since the actual "is this a Task" gate happens
// per-document inside ExtractScripts (a file can contain a Task doc mixed
// with other kinds in a multi-document stream).
func FilterTektonTasks(files []string) []string {
	var out []string
	for _, f := range files {
		if isYAMLFile(f) {
			out = append(out, f)
		}
	}
	return out
}

func isYAMLFile(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml")
}

// ExtractScripts parses a single Task YAML (possibly multi-doc) and returns
// one TektonScript per spec.steps[].script that isBashScript. Non-bash
// steps (python, no script field) are silently skipped.
func ExtractScripts(path string) ([]TektonScript, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return extractScriptsFromBytes(data, path)
}

func extractScriptsFromBytes(data []byte, path string) ([]TektonScript, error) {
	var scripts []TektonScript
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
		if quickString(findKey(mapping, "kind")) != "Task" {
			continue
		}
		taskName := quickName(mapping)
		spec := findKey(mapping, "spec")
		if spec == nil || spec.Kind != yaml.MappingNode {
			continue
		}
		steps := findKey(spec, "steps")
		if steps == nil || steps.Kind != yaml.SequenceNode {
			continue
		}
		for _, step := range steps.Content {
			if step.Kind != yaml.MappingNode {
				continue
			}
			stepName := quickString(findKey(step, "name"))
			scriptNode := findKey(step, "script")
			if scriptNode == nil || scriptNode.Kind != yaml.ScalarNode {
				continue
			}
			if !isBashScript(scriptNode.Value) {
				continue
			}
			scripts = append(scripts, TektonScript{
				File: path, TaskName: taskName, StepName: stepName,
				Script:     scriptNode.Value,
				LineOffset: scalarContentLine(scriptNode),
			})
		}
	}
	return scripts, nil
}

// scalarContentLine returns the line number (1-indexed, in the source
// file) where a scalar node's actual content begins. For block-style
// scalars ("|" literal or ">" folded - the style every real embedded
// script uses), yaml.v3 reports Node.Line as the line of the block-scalar
// indicator itself ("script: |"), not its first content line, so the
// content always begins one line after that. Plain/quoted scalars have no
// such offset - their content starts on the reported line itself.
func scalarContentLine(n *yaml.Node) int {
	if n.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
		return n.Line + 1
	}
	return n.Line
}

// RunTekton shellchecks every extracted bash step script across files,
// adjusting shellcheck's "In <tempfile> line N:" output back to the
// original YAML's line number via LineOffset.
func RunTekton(files []string) ([]TektonResult, error) {
	var results []TektonResult
	for _, f := range FilterTektonTasks(files) {
		scripts, err := ExtractScripts(f)
		if err != nil {
			continue
		}
		for _, ts := range scripts {
			violations, output, tmpPath, err := runShellcheckOnScript(ts.Script)
			if err != nil {
				if errors.Is(err, ErrCLINotFound) {
					return results, err
				}
				continue
			}
			adjViolations, adjOutput := finalizeScriptResult(violations, output, tmpPath, ts.File, ts.LineOffset)
			results = append(results, TektonResult{
				File: ts.File, TaskName: ts.TaskName, StepName: ts.StepName,
				Violations: adjViolations,
				Output:     adjOutput,
			})
		}
	}
	return results, nil
}

// runShellcheckOnScript writes script to a temp file and shellchecks it,
// returning the raw (temp-file-relative) findings, raw output, and the
// temp path used (so the caller can adjust both back to the original
// file's line numbers via finalizeScriptResult).
func runShellcheckOnScript(script string) (violations []Violation, output, tmpPath string, err error) {
	tmp, err := os.CreateTemp("", "shellcheck-*.sh")
	if err != nil {
		return nil, "", "", err
	}
	tmpPath = tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.WriteString(script); err != nil {
		tmp.Close()
		return nil, "", "", err
	}
	if err := tmp.Close(); err != nil {
		return nil, "", "", err
	}
	violations, output, err = Run([]string{tmpPath})
	if err != nil {
		return nil, "", "", err
	}
	return violations, output, tmpPath, nil
}

// finalizeScriptResult rewrites both the parsed Violations and the raw
// output text so they consistently reflect the original source file and
// its line numbers instead of the (random, meaningless-outside-this-run)
// temp file path: Violation.File/Line are rewritten directly, and every
// "tmpPath:N:" reference in output is rewritten to
// "<extracted-script>:N':" via adjustLineRef, so a PR comment never leaks
// a temp filename and both views agree on line numbers.
func finalizeScriptResult(violations []Violation, output, tmpPath, file string, lineOffset int) (adjustedViolations []Violation, adjustedOutput string) {
	adjusted := make([]Violation, len(violations))
	for i, v := range violations {
		v.File = file
		if lineOffset > 0 {
			v.Line = v.Line + lineOffset - 1
		}
		adjusted[i] = v
	}
	return adjusted, adjustLineRef(output, tmpPath, "<extracted-script>", lineOffset)
}

// adjustLineRef rewrites every occurrence of "tmpPath:N:" in output to
// "placeholder:N':" where N' = N + lineOffset - 1 (or just N, unchanged,
// when lineOffset <= 0 - the zero-offset passthrough case). A line that
// doesn't reference tmpPath at all is returned unmodified.
func adjustLineRef(output, tmpPath, placeholder string, lineOffset int) string {
	if output == "" {
		return output
	}
	re := regexp.MustCompile("(" + regexp.QuoteMeta(tmpPath) + `):(\d+):`)
	return re.ReplaceAllStringFunc(output, func(m string) string {
		sub := re.FindStringSubmatch(m)
		if len(sub) != 3 {
			return m
		}
		n, err := strconv.Atoi(sub[2])
		if err != nil {
			return m
		}
		if lineOffset > 0 {
			n = n + lineOffset - 1
		}
		return placeholder + ":" + strconv.Itoa(n) + ":"
	})
}

func quickString(n *yaml.Node) string {
	if n == nil {
		return ""
	}
	return n.Value
}

func quickName(mapping *yaml.Node) string {
	if meta := findKey(mapping, "metadata"); meta != nil && meta.Kind == yaml.MappingNode {
		if n := quickString(findKey(meta, "name")); n != "" {
			return n
		}
	}
	return "(unnamed)"
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
