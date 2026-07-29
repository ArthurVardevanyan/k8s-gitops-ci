package ghostpatch

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Target identifies a kustomize patch target.
type Target struct {
	Kind      string
	Name      string
	Namespace string
}

func (t Target) String() string {
	kind := t.Kind
	if kind == "" {
		kind = "<unknown>"
	}
	name := t.Name
	if name == "" {
		name = "<all>"
	}
	s := fmt.Sprintf("%s/%s", kind, name)
	if t.Namespace != "" {
		s += fmt.Sprintf(" (ns: %s)", t.Namespace)
	}
	return s
}

// Resource models a rendered Kubernetes resource.
type Resource struct {
	Kind     string
	Metadata Metadata
}

// Metadata holds resource identity.
type Metadata struct {
	Name      string
	Namespace string
}

// GhostPatch pairs a target with its overlay.
type GhostPatch struct {
	Target  Target
	Overlay string
}

// GhostResult is a classified ghost patch finding.
type GhostResult struct {
	Target   Target
	Blocking bool
}

// AppOverlayResult holds per-overlay ghost check results.
type AppOverlayResult struct {
	Overlay string
	Ghosts  []GhostResult
	Err     error
}

// CheckOverlay checks a single overlay's kustomization for ghost patches.
func CheckOverlay(overlayPath, renderedYAML string) ([]Target, error) {
	kustPath := filepath.Join(overlayPath, "kustomization.yaml")
	if _, err := os.Stat(kustPath); err != nil {
		return nil, nil
	}
	data, err := os.ReadFile(kustPath)
	if err != nil {
		return nil, err
	}
	var kust Kustomization
	if err := yaml.Unmarshal(data, &kust); err != nil {
		return nil, err
	}
	if len(kust.Patches) == 0 {
		return nil, nil
	}

	resources := parseResources(renderedYAML)
	renames := computeRenames(kust.Patches)
	var ghosts []Target
	for _, p := range kust.Patches {
		if strings.Contains(p.Patch, "$patch: delete") {
			continue
		}
		t := p.Target
		if t.Kind == "" && t.Name == "" {
			continue
		}
		lookupName := renames[targetKey(t)]
		if lookupName == "" {
			lookupName = t.Name
		}
		if !exists(resources, t.Kind, lookupName, t.Namespace) {
			ghosts = append(ghosts, t)
		}
	}
	return ghosts, nil
}

// CheckApp checks all overlays under an app.
func CheckApp(appPath string) ([]AppOverlayResult, error) {
	dir := filepath.Join(appPath, "overlays")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var results []AppOverlayResult
	for _, e := range entries {
		if e.IsDir() {
			ov := filepath.Join(dir, e.Name())
			if _, err := os.Stat(filepath.Join(ov, "kustomization.yaml")); err != nil {
				continue
			}
			rendered, err := kustomizeBuild(ov)
			if err != nil {
				results = append(results, AppOverlayResult{Overlay: ov, Err: err})
				continue
			}
			ghosts, err := CheckOverlay(ov, rendered)
			if err != nil {
				results = append(results, AppOverlayResult{Overlay: ov, Err: err})
				continue
			}
			var gr []GhostResult
			for _, g := range ghosts {
				gr = append(gr, GhostResult{Target: g})
			}
			results = append(results, AppOverlayResult{Overlay: ov, Ghosts: gr})
		}
	}
	return results, nil
}

// ClassifyOverlay classifies ghosts as blocking or warning.
func ClassifyOverlay(overlayPath, renderedYAML string, addedFiles []string) ([]GhostResult, error) {
	ghosts, err := CheckOverlay(overlayPath, renderedYAML)
	if err != nil {
		return nil, err
	}
	kustPath := filepath.Join(overlayPath, "kustomization.yaml")
	isNew := false
	for _, a := range addedFiles {
		if a == kustPath || strings.HasSuffix(a, filepath.ToSlash(kustPath)) {
			isNew = true
			break
		}
	}
	sectionChanged, _ := PatchesSectionChanged(kustPath)
	var out []GhostResult
	for _, g := range ghosts {
		blocking := false
		if !isNew && sectionChanged {
			blocking = true
		}
		out = append(out, GhostResult{Target: g, Blocking: blocking})
	}
	return out, nil
}

// PatchesSectionChanged compares the current patches section to main.
func PatchesSectionChanged(kustPath string) (bool, error) {
	current, err := os.ReadFile(kustPath)
	if err != nil {
		return false, err
	}
	currentPatches := extractPatchesYAML(current)

	base, err := gitShow(context.Background(), "main", kustPath)
	if err != nil {
		base, err = gitShow(context.Background(), "origin/main", kustPath)
		if err != nil {
			// new file or git unavailable -> not changed
			return false, nil
		}
	}
	basePatches := extractPatchesYAML(base)
	return string(currentPatches) != string(basePatches), nil
}

// Kustomization represents a minimal kustomization.yaml.
type Kustomization struct {
	Patches []Patch `yaml:"patches"`
}

// Patch represents a kustomize patch entry.
type Patch struct {
	Target Target `yaml:"target"`
	Patch  string `yaml:"patch"`
}

func parseResources(yamlStr string) []Resource {
	var resources []Resource
	dec := yaml.NewDecoder(strings.NewReader(yamlStr))
	for {
		var doc map[string]any
		if err := dec.Decode(&doc); err != nil {
			break
		}
		if kind, ok := doc["kind"].(string); ok {
			var meta Metadata
			if m, ok := doc["metadata"].(map[string]any); ok {
				meta.Name, _ = m["name"].(string)
				meta.Namespace, _ = m["namespace"].(string)
			}
			resources = append(resources, Resource{Kind: kind, Metadata: meta})
		}
	}
	return resources
}

func computeRenames(patches []Patch) map[string]string {
	renames := make(map[string]string)
	for _, p := range patches {
		newName := renameFromPatch(p.Patch)
		if newName == "" {
			continue
		}
		key := targetKey(p.Target)
		renames[key] = newName
	}
	return renames
}

var renameRe = regexp.MustCompile(`"op"\s*:\s*"(replace|add)"[^}]*"path"\s*:\s*"/?metadata/name"[^}]*"value"\s*:\s*"([^"]+)"`)

func renameFromPatch(patch string) string {
	m := renameRe.FindStringSubmatch(strings.ReplaceAll(patch, "\n", " "))
	if len(m) >= 3 {
		return m[2]
	}
	return ""
}

func exists(resources []Resource, kind, name, namespace string) bool {
	for _, r := range resources {
		if kind != "" && r.Kind != kind {
			continue
		}
		if namespace != "" && r.Metadata.Namespace != namespace {
			continue
		}
		if name != "" && r.Metadata.Name == name {
			return true
		}
	}
	return false
}

func targetKey(t Target) string {
	return fmt.Sprintf("%s/%s/%s", t.Kind, t.Name, t.Namespace)
}

func kustomizeBuild(overlay string) (string, error) {
	out, err := exec.Command("kustomize", "build", overlay).Output()
	return string(out), err
}

func gitShow(ctx context.Context, ref, path string) ([]byte, error) {
	return exec.CommandContext(ctx, "git", "show", fmt.Sprintf("%s:%s", ref, path)).Output()
}

func extractPatchesYAML(data []byte) []byte {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil
	}
	if len(root.Content) == 0 {
		return nil
	}
	doc := root.Content[0]
	for i := 0; i < len(doc.Content); i += 2 {
		if doc.Content[i].Value == "patches" {
			var buf strings.Builder
			enc := yaml.NewEncoder(&buf)
			enc.SetIndent(2)
			_ = enc.Encode(doc.Content[i+1])
			_ = enc.Close()
			return []byte(buf.String())
		}
	}
	return nil
}
