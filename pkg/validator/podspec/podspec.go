package podspec

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const Marker = "<!-- podspec-defaults-warning -->"

// WorkloadKinds lists kinds that contain a pod template.
var WorkloadKinds = map[string]bool{
	"Deployment": true, "StatefulSet": true, "DaemonSet": true, "Job": true, "CronJob": true, "Pod": true,
}

// RequiredPodFields lists required pod-level fields.
var RequiredPodFields = []string{
	"enableServiceLinks", "restartPolicy", "schedulerName", "dnsPolicy",
	"automountServiceAccountToken",
}

// RequiredSecurityContextFields lists required container securityContext fields.
var RequiredSecurityContextFields = []string{
	"allowPrivilegeEscalation", "readOnlyRootFilesystem", "privileged", "runAsNonRoot",
	"capabilities", "seccompProfile",
}

// ValidationError records a missing pod spec or securityContext field.
type ValidationError struct {
	File, Name, Kind, Container string
	MissingFields               []string
	Path                        string
}

func (e ValidationError) String() string {
	loc := ""
	if e.Path != "" {
		loc = " at " + e.Path
	}
	fields := strings.Join(e.MissingFields, ", ")
	if e.Container != "" {
		return fmt.Sprintf("%s: %s %q container %q missing securityContext fields: [%s]%s", e.File, e.Kind, e.Name, e.Container, fields, loc)
	}
	return fmt.Sprintf("%s: %s %q missing pod spec fields: [%s]%s", e.File, e.Kind, e.Name, fields, loc)
}

// DeduplicatedError aggregates podspec findings.
type DeduplicatedError struct {
	Kind, Name, Container string
	MissingFields, Files  []string
	Count                 int
	Path                  string
}

func (d DeduplicatedError) String() string {
	return fmt.Sprintf("%s %q missing %s (%d overlay(s))", d.Kind, d.Name, strings.Join(d.MissingFields, ", "), d.Count)
}

// ValidateFile validates pod spec / securityContext fields in a file.
func ValidateFile(path string) []ValidationError {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	return ValidateReader(f, path)
}

// ValidateReader validates pod spec / securityContext fields from a reader.
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
		podPath := "spec"
		if kind != "Pod" {
			podPath = "spec.template.spec"
		}
		podSpec := getNodeAtPath(mapping, podPath)
		if podSpec == nil || podSpec.Kind != yaml.MappingNode {
			continue
		}
		errs = append(errs, validatePodFields(source, kind, name, podSpec, podPath)...)
		errs = append(errs, validateContainerFields(source, kind, name, podSpec, podPath)...)
	}
	return errs
}

// FormatComment renders podspec findings as a PR comment block.
func FormatComment(errs []ValidationError) string {
	if len(errs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(Marker + "\n")
	b.WriteString("### Pod Spec / Security Context Fields Missing\n\n")
	b.WriteString("| Resource | Missing Fields |\n")
	b.WriteString("| --- | --- |\n")
	for _, e := range errs {
		b.WriteString("| ")
		b.WriteString(e.Kind)
		b.WriteString("/")
		b.WriteString(e.Name)
		b.WriteString(" | ")
		b.WriteString(strings.Join(e.MissingFields, ", "))
		b.WriteString(" |\n")
	}
	b.WriteString("\n<details><summary>Recommended defaults</summary>\n\n")
	b.WriteString("```yaml\n")
	b.WriteString("spec:\n  enableServiceLinks: false\n  automountServiceAccountToken: false\n  securityContext:\n    runAsNonRoot: true\n  containers:\n  - securityContext:\n      allowPrivilegeEscalation: false\n      readOnlyRootFilesystem: true\n      privileged: false\n      runAsNonRoot: true\n      capabilities:\n        drop:\n        - ALL\n      seccompProfile:\n        type: RuntimeDefault\n```\n")
	b.WriteString("</details>\n")
	return b.String()
}

// FormatDeduplicatedComment renders deduplicated findings.
func FormatDeduplicatedComment(deduped []DeduplicatedError) string {
	if len(deduped) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(Marker + "\n")
	b.WriteString("### Pod Spec / Security Context Fields Missing\n\n")
	b.WriteString("| Resource | Path | Missing Fields | Overlays |\n")
	b.WriteString("| --- | --- | --- | --- |\n")
	for _, d := range deduped {
		fmt.Fprintf(&b, "| %s/%s | %s | %s | %d |\n", d.Kind, d.Name, d.Path, strings.Join(d.MissingFields, ", "), d.Count)
	}
	b.WriteString("\n<details><summary>Recommended defaults</summary>\n\n")
	b.WriteString("```yaml\n")
	b.WriteString("spec:\n  enableServiceLinks: false\n  automountServiceAccountToken: false\n  containers:\n  - securityContext:\n      allowPrivilegeEscalation: false\n      readOnlyRootFilesystem: true\n      privileged: false\n      runAsNonRoot: true\n      capabilities:\n        drop:\n        - ALL\n      seccompProfile:\n        type: RuntimeDefault\n```\n")
	b.WriteString("</details>\n")
	return b.String()
}

// Deduplicate aggregates podspec findings, capping Files.
func Deduplicate(errors []ValidationError, maxFiles int) []DeduplicatedError {
	if maxFiles <= 0 {
		maxFiles = 3
	}
	seen := make(map[string]*DeduplicatedError)
	order := make([]string, 0, len(errors))
	for _, e := range errors {
		key := e.Kind + "/" + e.Name + "/" + e.Container + "/" + strings.Join(e.MissingFields, ",") + "/" + e.Path
		if d, ok := seen[key]; ok {
			d.Count++
			if len(d.Files) < maxFiles && !contains(d.Files, e.File) {
				d.Files = append(d.Files, e.File)
			}
			continue
		}
		seen[key] = &DeduplicatedError{
			Kind: e.Kind, Name: e.Name, Container: e.Container, Path: e.Path,
			MissingFields: append([]string{}, e.MissingFields...),
			Files:         []string{e.File}, Count: 1,
		}
		order = append(order, key)
	}
	out := make([]DeduplicatedError, 0, len(order))
	for _, k := range order {
		out = append(out, *seen[k])
	}
	return out
}

func validatePodFields(source, kind, name string, podSpec *yaml.Node, podPath string) []ValidationError {
	var errs []ValidationError
	var missing []string
	for _, f := range RequiredPodFields {
		if findKey(podSpec, f) == nil {
			missing = append(missing, f)
		}
	}
	if len(missing) > 0 {
		errs = append(errs, ValidationError{
			File: source, Kind: kind, Name: name, MissingFields: missing, Path: podPath,
		})
	}
	return errs
}

func validateContainerFields(source, kind, name string, podSpec *yaml.Node, podPath string) []ValidationError {
	var errs []ValidationError
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
			sec := findKey(cont, "securityContext")
			if sec == nil || sec.Kind != yaml.MappingNode {
				path := fmt.Sprintf("%s.%s[].securityContext", podPath, listKey)
				errs = append(errs, ValidationError{
					File: source, Kind: kind, Name: name, Container: cname,
					MissingFields: RequiredSecurityContextFields, Path: path,
				})
			} else {
				var missing []string
				for _, f := range RequiredSecurityContextFields {
					if findKey(sec, f) == nil {
						missing = append(missing, f)
					}
				}
				if len(missing) > 0 {
					path := fmt.Sprintf("%s.%s[].securityContext", podPath, listKey)
					errs = append(errs, ValidationError{
						File: source, Kind: kind, Name: name, Container: cname,
						MissingFields: missing, Path: path,
					})
				}
			}
			if findKey(cont, "resources") == nil {
				path := fmt.Sprintf("%s.%s[]", podPath, listKey)
				errs = append(errs, ValidationError{
					File: source, Kind: kind, Name: name, Container: cname,
					MissingFields: []string{"resources.requests", "resources.limits"}, Path: path,
				})
			}
		}
	}
	return errs
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
