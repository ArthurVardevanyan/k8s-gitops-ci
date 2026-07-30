package clusterid

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/cluster"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/exempt"
)

// ClusterTokenRe is an optional cluster-name token regex (nil = off).
var ClusterTokenRe *regexp.Regexp

// AllowField reports whether a field name should be skipped during identity
// scanning (e.g. "clusterName" in a resource that legitimately uses it).
// Org layers may replace this with a custom matcher.
var AllowField func(field string) bool

// ValidationError records a cluster identity finding.
type ValidationError struct {
	File    string
	Message string
	Direct  bool
}

func (e ValidationError) String() string {
	return fmt.Sprintf("%s: %s", e.File, e.Message)
}

// OverlayIdentity captures tokens found in an overlay.
type OverlayIdentity struct {
	ClusterName    string
	ProjectIDs     []string
	ProjectNumbers []string
	InfraIDs       []string
	ClusterNames   []string
	Sources        map[string][]string
}

// ClusterIndex maps project ids/numbers to cluster names.
type ClusterIndex struct {
	IDToCluster     map[string]string
	NumberToCluster map[string]string
	KnownClusters   map[string]bool
}

// Options configures cluster identity validation.
type Options struct {
	Index      ClusterIndex
	Selectors  []exempt.Selector
	DirectFile func(path string) bool
}

// Finding is a registry-facing finding.
type Finding struct {
	CheckID, File, Field, Value, Token, Kind, Name, Namespace string
	Annotations                                               map[string]string
	Message                                                   string
}

// AppliedExemption records an accepted exemption.
type AppliedExemption struct {
	CheckID, File, Field, Value, Token string
	Direct                             bool
}

// WIFCredentials identifies credentials shape used for foreign detection.
type WIFCredentials struct {
	Audience                       string
	ServiceAccountImpersonationURL string
}

var (
	projectNumberRe = regexp.MustCompile(`projects/(\d+)/locations/`)
	projectIDRe     = regexp.MustCompile(`@([^.]+)\.iam\.gserviceaccount\.com`)
	tokenSeparator  = regexp.MustCompile(`[^A-Za-z0-9_-]+`)
	// infraIDRe matches an "infraID"/"infra_id" key (YAML, JSON, or shell
	// env-var assignment style) and captures its value.
	infraIDRe = regexp.MustCompile(`(?i)"?infra[_-]?id"?\s*[:=]\s*"?([A-Za-z0-9][A-Za-z0-9._-]*)"?`)
)

// RawFindings returns registry-facing findings for an overlay.
func RawFindings(overlayPath, clusterName string, index ClusterIndex) []Finding {
	var findings []Finding
	identity := GetIdentity(overlayPath, clusterName)
	for _, num := range identity.ProjectNumbers {
		if owner, ok := index.NumberToCluster[num]; ok && owner != clusterName {
			findings = append(findings, Finding{
				CheckID: exempt.IDProjectRef, File: overlayPath,
				Field: "projectNumber", Value: num,
				Message: fmt.Sprintf("project number %q belongs to cluster %q but is referenced in the overlay for %q (likely a copy/paste error)", num, owner, clusterName),
			})
		}
	}
	for _, id := range identity.ProjectIDs {
		if owner, ok := index.IDToCluster[id]; ok && owner != clusterName {
			findings = append(findings, Finding{
				CheckID: exempt.IDProjectRef, File: overlayPath,
				Field: "projectID", Value: id,
				Message: fmt.Sprintf("project id %q belongs to cluster %q but is referenced in the overlay for %q (likely a copy/paste error)", id, owner, clusterName),
			})
		}
	}
	for _, foreign := range identity.ClusterNames {
		if foreign != clusterName {
			findings = append(findings, Finding{
				CheckID: exempt.IDClusterName, File: overlayPath,
				Value:   foreign,
				Message: fmt.Sprintf("cluster name %q belongs to a different cluster but appears in the overlay for %q (likely a copy/paste error)", foreign, clusterName),
			})
		}
	}
	findings = append(findings, rawInfraIDFindings(overlayPath, identity)...)
	return findings
}

// rawInfraIDFindings emits a non-exemptable structural finding for every
// infraID that doesn't match the overlay's own folder name. Unlike
// project-ref/cluster-name findings, this uses exempt.IDClusterIdentity
// (never exemptable) since a mismatched infraID indicates the overlay was
// very likely copy/pasted wholesale from another cluster's folder - the
// kind of error EXEMPTIONS selectors and annotations aren't meant to paper
// over.
func rawInfraIDFindings(overlayPath string, identity *OverlayIdentity) []Finding {
	findings := make([]Finding, 0, len(identity.InfraIDs))
	for _, infraID := range identity.InfraIDs {
		if infraID == identity.ClusterName {
			continue
		}
		findings = append(findings, Finding{
			CheckID: exempt.IDClusterIdentity, File: overlayPath,
			Field: "infraID", Value: infraID,
			Message: fmt.Sprintf("infraID %q does not match overlay folder name %q (likely a copy/paste error)", infraID, identity.ClusterName),
		})
	}
	return findings
}

// GetIdentity extracts identity tokens from an overlay.
func GetIdentity(overlayPath, clusterName string) *OverlayIdentity {
	id := &OverlayIdentity{ClusterName: clusterName, Sources: make(map[string][]string)}
	id.ClusterName = selfClusterName(clusterName)
	_ = filepath.Walk(overlayPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if info != nil && info.IsDir() && info.Name() == "overlays" {
				return filepath.SkipDir
			}
			return nil //nolint:nilerr // filepath.Walk convention: skip entry, keep walking
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil //nolint:nilerr // skip unreadable file, keep walking
		}
		rel, _ := filepath.Rel(overlayPath, path)
		id.scanString(string(data), rel)
		return nil
	})
	id.ProjectIDs = uniqueSorted(id.ProjectIDs)
	id.ProjectNumbers = uniqueSorted(id.ProjectNumbers)
	id.InfraIDs = uniqueSorted(id.InfraIDs)
	id.ClusterNames = uniqueSorted(id.ClusterNames)
	return id
}

func appendUnique(sl []string, s string) []string {
	for _, x := range sl {
		if x == s {
			return sl
		}
	}
	return append(sl, s)
}

func (id *OverlayIdentity) scanString(s, source string) {
	for _, m := range projectNumberRe.FindAllStringSubmatch(s, -1) {
		if AllowField != nil && AllowField("projectNumber") {
			continue
		}
		id.ProjectNumbers = append(id.ProjectNumbers, m[1])
		id.Sources[m[1]] = append(id.Sources[m[1]], source)
	}
	for _, m := range projectIDRe.FindAllStringSubmatch(s, -1) {
		if AllowField != nil && AllowField("projectID") {
			continue
		}
		id.ProjectIDs = append(id.ProjectIDs, m[1])
		id.Sources[m[1]] = append(id.Sources[m[1]], source)
	}
	for _, m := range infraIDRe.FindAllStringSubmatch(s, -1) {
		if AllowField != nil && AllowField("infraID") {
			continue
		}
		id.InfraIDs = append(id.InfraIDs, m[1])
		id.Sources[m[1]] = append(id.Sources[m[1]], source)
	}
	if ClusterTokenRe != nil {
		for _, tok := range tokenSeparator.Split(s, -1) {
			if AllowField != nil && AllowField("clusterName") {
				continue
			}
			if ClusterTokenRe.MatchString(tok) && tok != id.ClusterName {
				id.ClusterNames = appendUnique(id.ClusterNames, tok)
				id.Sources[tok] = append(id.Sources[tok], source)
			}
		}
	}
}

// FormatIdentity renders an OverlayIdentity summary.
func FormatIdentity(id *OverlayIdentity) string {
	var b strings.Builder
	fmt.Fprintf(&b, "cluster: %s\n", id.ClusterName)
	b.WriteString("project ids: ")
	b.WriteString(strings.Join(id.ProjectIDs, ", "))
	b.WriteString("\nproject numbers: ")
	b.WriteString(strings.Join(id.ProjectNumbers, ", "))
	b.WriteString("\n")
	return b.String()
}

// SelfClusterName strips prefix up to last '_'.
func SelfClusterName(name string) string {
	return selfClusterName(name)
}

func selfClusterName(name string) string {
	if idx := strings.LastIndex(name, "_"); idx != -1 {
		return name[idx+1:]
	}
	return name
}

func uniqueSorted(sl []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, s := range sl {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

var _ = yaml.Marshal

// BuildClusterIndex converts cluster.ProjectIndex to ClusterIndex.
func BuildClusterIndex(idx cluster.ProjectIndex, known map[string]bool) ClusterIndex {
	return ClusterIndex{
		IDToCluster:     idx.IDToCluster,
		NumberToCluster: idx.NumberToCluster,
		KnownClusters:   known,
	}
}
