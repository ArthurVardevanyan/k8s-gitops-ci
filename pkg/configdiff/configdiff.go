package configdiff

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/convention"
	"gopkg.in/yaml.v3"
)

// EnvironmentPrefixes maps environment names to cluster-name prefixes.
// Orgs override; empty default means no environment expansion.
var EnvironmentPrefixes = map[string][]string{}

// AffectedApp describes apps and clusters affected by a config change.
type AffectedApp struct {
	App      string
	Clusters []string
	Trigger  string
	FullTest bool
}

// DetectAffectedApps maps changed config files to affected clusters.
func DetectAffectedApps(changedFiles []string, repoURL, _ string, changeGroups map[string]int) []AffectedApp {
	prefix := convention.ScaffoldConfigsPrefix()
	appsMap := make(map[string]bool)
	for _, f := range changedFiles {
		if strings.HasPrefix(f, prefix) {
			app := appName(f[len(prefix):])
			if app != "" {
				appsMap[app] = true
			}
		}
	}
	if len(appsMap) == 0 {
		return nil
	}
	var out []AffectedApp
	for app := range appsMap {
		if affected := detectApp(app, changeGroups); len(affected.Clusters) > 0 || affected.FullTest || affected.Trigger != "" {
			out = append(out, affected)
		}
	}
	sortAffected(out)
	return out
}

// DetectTemplateChanges returns apps with changed scaffold templates.
func DetectTemplateChanges(changedFiles []string) []string {
	prefix := convention.ScaffoldTemplatesPrefix()
	seen := make(map[string]bool)
	var out []string
	for _, f := range changedFiles {
		if strings.HasPrefix(f, prefix) {
			rest := f[len(prefix):]
			first := ""
			if idx := strings.Index(rest, "/"); idx != -1 {
				first = rest[:idx]
			} else {
				first = rest
			}
			if first != "" && !seen[first] {
				seen[first] = true
				out = append(out, first)
			}
		}
	}
	return out
}

func detectApp(app string, changeGroups map[string]int) AffectedApp {
	configPath := findConfigFile(app)
	if configPath == "" {
		return AffectedApp{}
	}
	overlaysDir := filepath.Join(app, "overlays")
	if _, err := os.Stat(overlaysDir); err != nil {
		return AffectedApp{}
	}
	current, err := os.ReadFile(configPath)
	if err != nil {
		return AffectedApp{}
	}
	mainConfig := fetchMainConfig(configPath)
	currentOverrides, currentEnvs, currentGroups := parseTopLevel(current)
	mainOverrides, mainEnvs, mainGroups := parseTopLevel(mainConfig)

	var clusters []string
	trigger := ""

	if OverridesDiff(mainOverrides, currentOverrides) {
		trigger = "config-override"
		for k := range currentOverrides {
			if _, ok := mainOverrides[k]; !ok || !yamlEqual(mainOverrides[k], currentOverrides[k]) {
				clusters = append(clusters, k)
			}
		}
	}

	envDiffs := envDiff(mainEnvs, currentEnvs)
	if len(envDiffs) > 0 {
		trigger = "config-environment"
		for _, env := range envDiffs {
			prefixes := EnvironmentPrefixes[env]
			for _, p := range prefixes {
				matches, _ := filepath.Glob(filepath.Join(overlaysDir, p+"*"))
				for _, m := range matches {
					clusters = appendUnique(clusters, filepath.Base(m))
				}
			}
		}
	}

	groupDiff := !yamlMappingEqual(mainGroups, currentGroups)
	fullTest := false
	if groupDiff && len(changeGroups) > 0 {
		trigger = "config-changeGroup"
		fullTest = true
		for cluster, group := range changeGroups {
			if group == 0 {
				continue
			}
			clusters = appendUnique(clusters, cluster)
		}
	}

	clusters = filterExisting(clusters, overlaysDir)
	clusters = dedup(clusters)
	return AffectedApp{App: app, Clusters: clusters, Trigger: trigger, FullTest: fullTest}
}

func findConfigFile(app string) string {
	for _, ext := range []string{".yaml", ".yml"} {
		p := filepath.Join(convention.ScaffoldDir, "configs", app+ext)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func appName(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, "/"); idx != -1 {
		s = s[:idx]
	}
	ext := filepath.Ext(s)
	if ext == ".yaml" || ext == ".yml" {
		return strings.TrimSuffix(s, ext)
	}
	return ""
}

func fetchMainConfig(path string) []byte {
	ctx := context.Background()
	base, _ := exec.CommandContext(ctx, "git", "merge-base", "HEAD", "origin/main").Output()
	ref := strings.TrimSpace(string(base))
	if ref == "" {
		ref = "origin/main"
	}
	rel := filepath.ToSlash(path)
	out, _ := exec.CommandContext(ctx, "git", "show", fmt.Sprintf("%s:%s", ref, rel)).Output()
	return out
}

func parseTopLevel(data []byte) (overrides map[string]*yaml.Node, envs map[string]*yaml.Node, groups map[string]*yaml.Node) {
	overrides = make(map[string]*yaml.Node)
	envs = make(map[string]*yaml.Node)
	groups = make(map[string]*yaml.Node)
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil || len(root.Content) == 0 {
		return
	}
	doc := root.Content[0]
	if od := findKey(doc, "overlayDefinitions"); od != nil {
		if ovs := findKey(od, "overrides"); ovs != nil {
			for i := 0; i < len(ovs.Content); i += 2 {
				overrides[ovs.Content[i].Value] = ovs.Content[i+1]
			}
		}
	}
	if e := findKey(doc, "environments"); e != nil {
		for i := 0; i < len(e.Content); i += 2 {
			envs[e.Content[i].Value] = e.Content[i+1]
		}
	}
	if g := findKey(doc, "changeGroups"); g != nil {
		for i := 0; i < len(g.Content); i += 2 {
			groups[g.Content[i].Value] = g.Content[i+1]
		}
	}
	return
}

func findKey(node *yaml.Node, key string) *yaml.Node {
	if node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

// OverridesDiff reports whether override maps differ.
func OverridesDiff(a, b map[string]*yaml.Node) bool {
	if len(a) != len(b) {
		return true
	}
	for k, va := range a {
		vb, ok := b[k]
		if !ok || !yamlEqual(va, vb) {
			return true
		}
	}
	return false
}

func envDiff(a, b map[string]*yaml.Node) []string {
	var out []string
	for k, vb := range b {
		va, ok := a[k]
		if !ok || !yamlEqual(va, vb) {
			out = append(out, k)
		}
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			out = append(out, k)
		}
	}
	return out
}

func yamlEqual(a, b *yaml.Node) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Kind != b.Kind {
		return false
	}
	if a.Value != b.Value {
		return false
	}
	if len(a.Content) != len(b.Content) {
		return false
	}
	for i := range a.Content {
		if !yamlEqual(a.Content[i], b.Content[i]) {
			return false
		}
	}
	return true
}

func yamlMappingEqual(a, b map[string]*yaml.Node) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		vb, ok := b[k]
		if !ok || !yamlEqual(va, vb) {
			return false
		}
	}
	return true
}

func filterExisting(clusters []string, overlaysDir string) []string {
	var out []string
	for _, c := range clusters {
		if _, err := os.Stat(filepath.Join(overlaysDir, c)); err == nil {
			out = append(out, c)
		}
	}
	return out
}

func appendUnique(sl []string, s string) []string {
	for _, x := range sl {
		if x == s {
			return sl
		}
	}
	return append(sl, s)
}

func dedup(sl []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, s := range sl {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func sortAffected(sl []AffectedApp) {
	for i := range sl {
		sort.Strings(sl[i].Clusters)
	}
}
