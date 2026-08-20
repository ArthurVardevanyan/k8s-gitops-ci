package convention

import (
	"path/filepath"
	"strings"
)

// KnownNonManifestFiles lists YAML/YML basenames that, by strong
// ecosystem convention, are never Kubernetes manifests - they're
// configuration files for common Go-ecosystem tooling (Task, golangci-lint,
// GoReleaser, pre-commit) that happen to live at a repo root alongside
// real Kubernetes YAML. Without this, any changeset touching one of these
// (e.g. this repo's own Taskfile.yml) trips a permanent, unavoidable
// "missing 'kind' key" error in manifest validators - not a one-off
// exemption case, but a structural fact about the file that's permanently
// true.
//
// Exported (not a plain unexported constant list) so an org/consumer layer
// can extend it for its own tooling's YAML config files - e.g. from a
// Configure()-equivalent in its own main() - following the same
// core-data-plus-override-var shape used elsewhere in this repo (see
// docs/DEVELOPMENT.md's "core data + org-injectable override" pattern),
// even though here it's one shared map an org adds to directly rather than
// a separate override map consulted alongside a core one - there's no
// risk of an org's addition shadowing or conflicting with a core entry,
// since basenames are simply either known-non-manifest or not.
var KnownNonManifestFiles = map[string]bool{
	"Taskfile.yml":            true,
	"Taskfile.yaml":           true,
	".golangci.yml":           true,
	".golangci.yaml":          true,
	".goreleaser.yaml":        true,
	".goreleaser.yml":         true,
	"dependabot.yml":          true,
	"dependabot.yaml":         true,
	".pre-commit-config.yaml": true,
	".bulldozer.yml":          true,
	".bulldozer.yaml":         true,
	".policy.yml":             true,
	".policy.yaml":            true,
}

// IsKnownNonManifestFile reports whether path's basename is a known
// non-Kubernetes-manifest tooling config file (see KnownNonManifestFiles).
func IsKnownNonManifestFile(path string) bool {
	return KnownNonManifestFiles[filepath.Base(path)]
}

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
