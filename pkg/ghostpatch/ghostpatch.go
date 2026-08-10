package ghostpatch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/overlay"
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
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
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
	results := make([]AppOverlayResult, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
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
	return results, nil
}

// RenderedOverlay pairs an overlay's directory path with its already-built
// YAML manifest stream, so ClassifyRendered can classify ghost patches
// against overlays a caller has already rendered (e.g. the Build YAML
// phase's own per-overlay renders) instead of re-rendering them itself.
type RenderedOverlay struct {
	Path string
	YAML string
}

// ClassifyRendered classifies ghost patches across overlays whose YAML has
// already been rendered by the caller, via ClassifyOverlay per overlay -
// the plural counterpart to ClassifyOverlay, taking the overlay set and its
// renders directly rather than discovering and re-rendering every overlay
// under an app on disk (see ClassifyApp's history/removal: that scanned
// every overlay in an app's overlays/ directory and called kustomizeBuild
// on each one, which for an app with hundreds of overlays and a PR
// touching only a handful was a redundant full-repo re-render dominating
// the Build YAML phase's wall time - see docs/CI.md's Ghost Patch
// Detection section). Callers should pass only the overlays relevant to
// this run (typically the PR's changed/rendered overlays) - an overlay
// omitted here is simply never classified, matching the intent that a
// ghost patch on an overlay this PR didn't touch/build isn't this run's
// concern.
func ClassifyRendered(overlays []RenderedOverlay, changedFiles, addedFiles []string) ([]AppOverlayResult, error) {
	results := make([]AppOverlayResult, 0, len(overlays))
	for _, ov := range overlays {
		gr, err := ClassifyOverlay(ov.Path, ov.YAML, changedFiles, addedFiles)
		if err != nil {
			results = append(results, AppOverlayResult{Overlay: ov.Path, Err: err})
			continue
		}
		results = append(results, AppOverlayResult{Overlay: ov.Path, Ghosts: gr})
	}
	return results, nil
}

// ClassifyOverlay classifies ghosts as blocking or warning.
//
// A ghost is blocking only when this PR changed the overlay's own
// kustomization.yaml (it is in changedFiles but was not newly added). A ghost
// on an overlay this PR did not touch - pre-existing drift, often from a
// stale/advanced base - is surfaced as a non-blocking warning for visibility:
// it is not something this PR introduced, so it must not fail the run. A
// brand-new overlay's ghosts are likewise non-blocking (nothing shipped with
// them yet).
func ClassifyOverlay(overlayPath, renderedYAML string, changedFiles, addedFiles []string) ([]GhostResult, error) {
	ghosts, err := CheckOverlay(overlayPath, renderedYAML)
	if err != nil {
		return nil, err
	}
	kustPath := filepath.Join(overlayPath, "kustomization.yaml")
	blocking := containsFile(changedFiles, kustPath) && !containsFile(addedFiles, kustPath)
	out := make([]GhostResult, 0, len(ghosts))
	for _, g := range ghosts {
		out = append(out, GhostResult{Target: g, Blocking: blocking})
	}
	return out, nil
}

// containsFile reports whether files contains path, comparing against both the
// raw path and its slash-normalized form.
func containsFile(files []string, path string) bool {
	slashed := filepath.ToSlash(path)
	for _, f := range files {
		if f == path || f == slashed {
			return true
		}
	}
	return false
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

// jsonPatchOp is a single JSON6902-style patch operation, as it appears in
// a kustomize patches[].patch block. Despite the "JSON" in the name,
// kustomize accepts (and real overlays typically use) YAML list syntax for
// this field, e.g.:
//
//	patch: |-
//	  - op: replace
//	    path: /metadata/name
//	    value: new-name
type jsonPatchOp struct {
	Op    string    `yaml:"op"`
	Path  string    `yaml:"path"`
	Value yaml.Node `yaml:"value"`
}

// renameFromPatch parses patch as a YAML list of JSON6902 operations and
// returns the new name if it contains a replace/add of /metadata/name
// whose value is a scalar. This must decode real YAML (not just a
// JSON-object-literal regex) since that's the syntax kustomize patches
// actually use in practice - a JSON-bracket-and-quotes-only matcher would
// silently fail to detect renames in the common case. Value is decoded as a
// yaml.Node (rather than a string) so that an unrelated op carrying a
// non-scalar value - e.g. an `add` /spec/logging whose value is a map - does
// not fail the whole op-list decode and thereby hide an earlier
// `/metadata/name` rename.
func renameFromPatch(patch string) string {
	var ops []jsonPatchOp
	if err := yaml.Unmarshal([]byte(patch), &ops); err != nil {
		return ""
	}
	for _, op := range ops {
		if op.Op != "replace" && op.Op != "add" {
			continue
		}
		if op.Path != "/metadata/name" && op.Path != "metadata/name" {
			continue
		}
		if op.Value.Kind != yaml.ScalarNode {
			continue
		}
		var name string
		if err := op.Value.Decode(&name); err != nil {
			return ""
		}
		return name
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
		// A name-less target (e.g. `target: {kind: CustomResourceDefinition}`,
		// typically paired with a label/annotation selector) matches every
		// resource of that kind - it is NOT a ghost patch as long as at least
		// one such resource was rendered. Only require an exact name match
		// when the target actually specifies one.
		if name == "" || r.Metadata.Name == name {
			return true
		}
	}
	return false
}

func targetKey(t Target) string {
	return fmt.Sprintf("%s/%s/%s", t.Kind, t.Name, t.Namespace)
}

// kustomizeBuild builds the overlay using the native Kustomize SDK (the same
// engine as pkg/overlay), avoiding a runtime dependency on a `kustomize`
// binary being installed in the CI image.
func kustomizeBuild(overlayPath string) (string, error) {
	out, err := overlay.RenderKustomize(overlayPath)
	return string(out), err
}
