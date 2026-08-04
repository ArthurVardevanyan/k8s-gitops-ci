package convention

import (
	"path/filepath"
	"strings"
)

// ScaffoldDir is the root directory for scaffold-tool configs/templates.
// Org layers may override this at startup.
var ScaffoldDir = ".scafctl"

// ScaffoldConfigsPrefix returns the path prefix for scaffold config files.
func ScaffoldConfigsPrefix() string {
	return filepath.Join(ScaffoldDir, "configs") + "/"
}

// ScaffoldTemplatesPrefix returns the path prefix for scaffold template files.
func ScaffoldTemplatesPrefix() string {
	return filepath.Join(ScaffoldDir, "templates") + "/"
}

// IsScaffoldConfig reports whether path is a scaffold-tool config file
// (under <ScaffoldDir>/configs/). These are the scaffolding CLI's own input
// files, not Kubernetes manifests - they have no apiVersion/kind - so
// manifest validators (kubeconform, Kyverno) must skip them.
func IsScaffoldConfig(path string) bool {
	return strings.Contains(filepath.ToSlash(path), filepath.ToSlash(ScaffoldConfigsPrefix()))
}

// IsScaffoldTemplate reports whether path is a scaffold-tool template file
// (under <ScaffoldDir>/templates/). These are Go-templated source files, so
// they are frequently not even valid standalone YAML (unresolved {{ ... }}
// actions); both syntax and manifest validators must skip them.
func IsScaffoldTemplate(path string) bool {
	return strings.Contains(filepath.ToSlash(path), filepath.ToSlash(ScaffoldTemplatesPrefix()))
}

// IsScaffoldArtifact reports whether path is any scaffold-tool config or
// template file - i.e. a repo file that is not a Kubernetes manifest and
// should be excluded from manifest/YAML validation of raw changed files.
func IsScaffoldArtifact(path string) bool {
	return IsScaffoldConfig(path) || IsScaffoldTemplate(path)
}
