package validator

import (
	"bytes"
	"strings"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/clusterid"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/crb"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/exempt"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/image"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/namedport"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/namespace"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/placeholder"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/podspec"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/psa"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/rbac"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/syncopts"
)

// ClusterIndexProvider is a seam allowing the engine to inject a ClusterIndex
// before running overlay checks. Set once at pipeline startup - nil means
// the cluster-identity check is disabled (see clusterIdentityAdapter.
// CheckOverlay), which is the correct default for a generic run with no
// org cluster-identity metadata configured.
var ClusterIndexProvider func() clusterid.ClusterIndex

// configureClusterIdentityFromProviders wires opts.Providers.ProjectIdentity()
// (the provider.Providers.ClusterMetadata seam - see pkg/provider) into
// ClusterIndexProvider for the duration of this run, bridging
// cluster.ProjectIndex to clusterid.ClusterIndex. Called once from
// RunAll. Leaves (or resets) ClusterIndexProvider to nil - disabling the
// cluster-identity check entirely, rather than running it against an
// empty index - whenever no ClusterMetadata provider is configured,
// ProjectIdentity errors, or it reports itself as not enabled: a generic
// run with no org cluster-identity plugin installed must never produce
// cluster-identity findings (including its always-on infraID-mismatch and
// invalid-JSON structural findings, which don't otherwise depend on the
// index's contents at all).
func configureClusterIdentityFromProviders(opts Options) {
	projectIdx, knownClusters, ok, err := opts.Providers.ProjectIdentity()
	if !ok || err != nil {
		ClusterIndexProvider = nil
		return
	}
	idx := clusterid.ClusterIndex{
		IDToCluster:     projectIdx.IDToCluster,
		NumberToCluster: projectIdx.NumberToCluster,
		KnownClusters:   knownClusters,
	}
	ClusterIndexProvider = func() clusterid.ClusterIndex { return idx }
}

func init() {
	// kubeconform is a standalone lint step (not a check.Register entry) but
	// is registered as exemptable so EXEMPTIONS=(check=kubeconform,...) entries
	// in test.sh can suppress kubeconform validation for specific files or
	// directories (e.g. non-Kubernetes YAML that has no kind/apiVersion).
	exempt.RegisterExemptable("kubeconform")

	check.Register(namespaceCheck{})
	check.Register(psaCheck{})
	check.Register(rbacReadonlyCheck{})
	check.Register(rbacWildcardCheck{})
	check.Register(crbCheck{})
	check.Register(syncoptsCheck{})
	check.Register(imageCheck{})
	check.Register(imageFQDNCheck{})
	check.Register(namedportCheck{})
	check.Register(podspecCheck{})
	check.Register(placeholderCheck{})
	check.Register(clusterIdentityAdapter{})
}

// ── namespace ────────────────────────────────────────────────────────────────

type namespaceCheck struct{}

func (namespaceCheck) ID() string            { return "namespace" }
func (namespaceCheck) Title() string         { return "Namespace Scope" }
func (namespaceCheck) Section() string       { return "resource-compliance" }
func (namespaceCheck) Blocking() bool        { return true }
func (namespaceCheck) Scope() check.Scope    { return check.ScopeDoc }
func (namespaceCheck) RenderSensitive() bool { return true }
func (namespaceCheck) CheckDoc(data []byte, source string) []check.Finding {
	errs := namespace.ValidateBytes(data, source)
	out := make([]check.Finding, 0, len(errs))
	for _, e := range errs {
		out = append(out, check.Finding{
			CheckID: "namespace", File: e.File,
			Kind: e.Kind, Name: e.Name, Message: e.Message,
		})
	}
	return out
}

// ── psa ──────────────────────────────────────────────────────────────────────

type psaCheck struct{}

func (psaCheck) ID() string            { return "psa-labels" }
func (psaCheck) Title() string         { return "PSA Namespace Labels" }
func (psaCheck) Section() string       { return "resource-compliance" }
func (psaCheck) Blocking() bool        { return true }
func (psaCheck) Scope() check.Scope    { return check.ScopeDoc }
func (psaCheck) RenderSensitive() bool { return true }
func (psaCheck) CheckDoc(data []byte, source string) []check.Finding {
	errs := psa.ValidateReader(bytes.NewReader(data), source)
	out := make([]check.Finding, 0, len(errs))
	for _, e := range errs {
		out = append(out, check.Finding{
			CheckID: "psa-labels", File: e.File,
			Name: e.Name, Message: e.String(),
			// MissingLabels is carried in Extra (rather than only inside
			// the formatted Message) so filterCommentedPSAFindings
			// (psa_wiring.go) can check each missing label individually
			// against psa.FindCommentedNamespaces without re-parsing the
			// rendered message string.
			Extra: map[string]string{psaMissingLabelsExtraKey: strings.Join(e.MissingLabels, ",")},
		})
	}
	return out
}

// ── rbac-readonly ─────────────────────────────────────────────────────────────

type rbacReadonlyCheck struct{}

func (rbacReadonlyCheck) ID() string            { return "rbac-readonly" }
func (rbacReadonlyCheck) Title() string         { return "RBAC Read-Only Aggregate" }
func (rbacReadonlyCheck) Section() string       { return "resource-compliance" }
func (rbacReadonlyCheck) Blocking() bool        { return true }
func (rbacReadonlyCheck) Scope() check.Scope    { return check.ScopeDoc }
func (rbacReadonlyCheck) RenderSensitive() bool { return true }
func (rbacReadonlyCheck) CheckDoc(data []byte, source string) []check.Finding {
	errs := rbac.ValidateReader(bytes.NewReader(data), source)
	out := make([]check.Finding, 0, len(errs))
	for _, e := range errs {
		out = append(out, check.Finding{
			CheckID: "rbac-readonly", File: e.File,
			Kind: e.Kind, Name: e.Resource, Message: e.String(),
		})
	}
	return out
}

// ── rbac-wildcards ────────────────────────────────────────────────────────────

type rbacWildcardCheck struct{}

func (rbacWildcardCheck) ID() string            { return "rbac-wildcards" }
func (rbacWildcardCheck) Title() string         { return "RBAC Wildcards" }
func (rbacWildcardCheck) Section() string       { return "resource-compliance" }
func (rbacWildcardCheck) Blocking() bool        { return true }
func (rbacWildcardCheck) Scope() check.Scope    { return check.ScopeDoc }
func (rbacWildcardCheck) RenderSensitive() bool { return true }
func (rbacWildcardCheck) CheckDoc(data []byte, source string) []check.Finding {
	errs := rbac.ValidateWildcardsReader(bytes.NewReader(data), source)
	out := make([]check.Finding, 0, len(errs))
	for _, e := range errs {
		out = append(out, check.Finding{
			CheckID: "rbac-wildcards", File: e.File,
			Kind: e.Kind, Name: e.Resource, Message: e.String(),
			// Value is the wildcarded rule field (e.g. "resources") -
			// shared across every rule index on the same
			// ClusterRole/Role that wildcards the same field, so one
			// annotation/selector exempts all of them at once.
			Value:       e.Field,
			Annotations: e.Annotations,
		})
	}
	return out
}

// ── crb ───────────────────────────────────────────────────────────────────────

type crbCheck struct{}

func (crbCheck) ID() string            { return "crb" }
func (crbCheck) Title() string         { return "ClusterRoleBinding Subject Namespace" }
func (crbCheck) Section() string       { return "resource-compliance" }
func (crbCheck) Blocking() bool        { return true }
func (crbCheck) Scope() check.Scope    { return check.ScopeDoc }
func (crbCheck) RenderSensitive() bool { return true }
func (crbCheck) CheckDoc(data []byte, source string) []check.Finding {
	errs := crb.ValidateBytes(data, source)
	out := make([]check.Finding, 0, len(errs))
	for _, e := range errs {
		out = append(out, check.Finding{
			CheckID: "crb", File: e.File,
			Kind: e.Kind, Name: e.Name, Message: e.Message,
		})
	}
	return out
}

// ── sync-options ──────────────────────────────────────────────────────────────

type syncoptsCheck struct{}

func (syncoptsCheck) ID() string            { return "sync-options" }
func (syncoptsCheck) Title() string         { return "Argo CD Sync Options" }
func (syncoptsCheck) Section() string       { return "resource-compliance" }
func (syncoptsCheck) Blocking() bool        { return true }
func (syncoptsCheck) Scope() check.Scope    { return check.ScopeDoc }
func (syncoptsCheck) RenderSensitive() bool { return true }
func (syncoptsCheck) CheckDoc(data []byte, source string) []check.Finding {
	errs := syncopts.ValidateReader(bytes.NewReader(data), source)
	out := make([]check.Finding, 0, len(errs))
	for _, e := range errs {
		out = append(out, check.Finding{
			CheckID: "sync-options", File: e.File,
			Kind: e.Kind, Name: e.Name, Message: e.String(),
		})
	}
	return out
}

// ── image-checksum ────────────────────────────────────────────────────────────

type imageCheck struct{}

func (imageCheck) ID() string            { return "image-checksum" }
func (imageCheck) Title() string         { return "Image Digest Pinning" }
func (imageCheck) Section() string       { return "resource-compliance" }
func (imageCheck) Blocking() bool        { return true }
func (imageCheck) Scope() check.Scope    { return check.ScopeDoc }
func (imageCheck) RenderSensitive() bool { return true }
func (imageCheck) CheckDoc(data []byte, source string) []check.Finding {
	// ValidateBytesRaw (not ValidateBytes) so exemption evaluation happens
	// once, uniformly, in the shared check/exempt engine - which is what
	// records an audit-trail entry for image-checksum exemptions and also
	// enables EXEMPTIONS-selector exemptions (ValidateBytes only ever
	// supported the annotation form, decided internally, before the
	// finding could reach this adapter at all).
	errs := image.ValidateBytesRaw(data, source)
	out := make([]check.Finding, 0, len(errs))
	for _, e := range errs {
		out = append(out, check.Finding{
			CheckID: "image-checksum", File: e.File,
			Kind: e.Kind, Name: e.Name,
			Value:       e.Image,
			Message:     e.Message,
			Annotations: e.Annotations,
		})
	}
	return out
}

// ── image-fqdn ────────────────────────────────────────────────────────────────

type imageFQDNCheck struct{}

func (imageFQDNCheck) ID() string            { return "image-fqdn" }
func (imageFQDNCheck) Title() string         { return "Image Registry FQDN" }
func (imageFQDNCheck) Section() string       { return "resource-compliance" }
func (imageFQDNCheck) Blocking() bool        { return true }
func (imageFQDNCheck) Scope() check.Scope    { return check.ScopeDoc }
func (imageFQDNCheck) RenderSensitive() bool { return true }
func (imageFQDNCheck) CheckDoc(data []byte, source string) []check.Finding {
	errs := image.ValidateFQDNBytesRaw(data, source)
	out := make([]check.Finding, 0, len(errs))
	for _, e := range errs {
		out = append(out, check.Finding{
			CheckID: "image-fqdn", File: e.File,
			Kind: e.Kind, Name: e.Name,
			Value:       e.Image,
			Message:     e.Message,
			Annotations: e.Annotations,
		})
	}
	return out
}

// ── named-ports ───────────────────────────────────────────────────────────────

type namedportCheck struct{}

func (namedportCheck) ID() string            { return "named-ports" }
func (namedportCheck) Title() string         { return "Named Ports" }
func (namedportCheck) Section() string       { return "resource-compliance" }
func (namedportCheck) Blocking() bool        { return true }
func (namedportCheck) Scope() check.Scope    { return check.ScopeDoc }
func (namedportCheck) RenderSensitive() bool { return true }
func (namedportCheck) CheckDoc(data []byte, source string) []check.Finding {
	errs := namedport.ValidateBytes(data, source)
	out := make([]check.Finding, 0, len(errs))
	for _, e := range errs {
		out = append(out, check.Finding{
			CheckID: "named-ports", File: e.File,
			Kind: e.Kind, Name: e.Name,
			Container: e.Container, Path: e.Path,
			Message:     e.Issue,
			Value:       e.Value,
			Annotations: e.Annotations,
		})
	}
	return out
}

// ── podspec-defaults ──────────────────────────────────────────────────────────

type podspecCheck struct{}

func (podspecCheck) ID() string            { return "podspec-defaults" }
func (podspecCheck) Title() string         { return "PodSpec Defaults" }
func (podspecCheck) Section() string       { return "resource-compliance" }
func (podspecCheck) Blocking() bool        { return true }
func (podspecCheck) Scope() check.Scope    { return check.ScopeDoc }
func (podspecCheck) RenderSensitive() bool { return true }
func (podspecCheck) CheckDoc(data []byte, source string) []check.Finding {
	errs := podspec.ValidateReader(bytes.NewReader(data), source)
	out := make([]check.Finding, 0, len(errs))
	for _, e := range errs {
		out = append(out, check.Finding{
			CheckID: "podspec-defaults", File: e.File,
			Kind: e.Kind, Name: e.Name,
			Container: e.Container, Path: e.Path,
			Message:     strings.Join(e.MissingFields, ", "),
			Value:       e.Value(),
			Annotations: e.Annotations,
		})
	}
	return out
}

// ── placeholder ───────────────────────────────────────────────────────────────

type placeholderCheck struct{}

func (placeholderCheck) ID() string            { return "placeholder" }
func (placeholderCheck) Title() string         { return "Unresolved Placeholders" }
func (placeholderCheck) Section() string       { return "resource-compliance" }
func (placeholderCheck) Blocking() bool        { return true }
func (placeholderCheck) Scope() check.Scope    { return check.ScopeDoc }
func (placeholderCheck) RenderSensitive() bool { return true }
func (placeholderCheck) CheckDoc(data []byte, source string) []check.Finding {
	// Raw-source pass. CheckAVP: false - AVP-scheme references
	// (<path:...#...>, <vault:...>, etc.) are the intended, committed state
	// in an argocd-vault-plugin repo: they are resolved by AVP at deploy
	// time (and by this pipeline's own overlay build via `argocd-vault-plugin
	// generate`), not by the author. Flagging them here would make every
	// AVP-managed secret a blocking false positive. Only genuine unresolved
	// template placeholders (angle-bracket UPPER/<a-b-c> tokens and CHANGEME
	// sentinels) are flagged.
	//
	// This raw pass only reaches files that never appear in any rendered
	// overlay (e.g. a brand-new component not yet wired into any
	// kustomization.yaml) - for every file that does participate in a
	// rendered overlay, the rendered pass (CheckRenderedDoc) is authoritative
	// and this raw pass's findings are suppressed. That is what prevents a
	// sentinel like `image: <PATCHED_BY_KUSTOMIZE>` in a base/component -
	// always replaced by an overlay `images:`/JSON-patch at build time - from
	// being reported as a false-positive unresolved placeholder.
	return placeholderFindings(placeholder.ValidateReaderWithOptions(
		bytes.NewReader(data), source, placeholder.Options{CheckAVP: false},
	))
}

// CheckRenderedDoc validates already-rendered (kustomize/AVP) output. Here
// CheckAVP: true, because a `<path:...>`/`<vault:...>` reference that
// SURVIVES AVP rendering is a genuine unresolved-secret failure (AVP was
// expected to resolve it and didn't), not the intended committed state.
func (placeholderCheck) CheckRenderedDoc(data []byte, source string) []check.Finding {
	return placeholderFindings(placeholder.ValidateReaderWithOptions(
		bytes.NewReader(data), source, placeholder.Options{CheckAVP: true},
	))
}

func placeholderFindings(errs []placeholder.ValidationError) []check.Finding {
	out := make([]check.Finding, 0, len(errs))
	for _, e := range errs {
		out = append(out, check.Finding{
			CheckID: "placeholder", File: e.File,
			Message: e.String(),
			Value:   e.Match,
		})
	}
	return out
}

// SkipDoc excludes CustomResourceDefinition documents: a CRD's embedded
// OpenAPI schema (defaults, examples, enum values, pattern strings, etc.)
// can legitimately contain angle-bracket/sentinel-shaped tokens - schema
// authoring conventions, not unresolved secrets - which would otherwise be
// misreported as unresolved placeholders.
func (placeholderCheck) SkipDoc(kind string) bool { return kind == "CustomResourceDefinition" }

// ── cluster-identity (overlay scope) ─────────────────────────────────────────

type clusterIdentityAdapter struct{}

func (clusterIdentityAdapter) ID() string         { return "cluster-identity" }
func (clusterIdentityAdapter) Title() string      { return "Cluster Identity Copy/Paste" }
func (clusterIdentityAdapter) Section() string    { return "resource-compliance" }
func (clusterIdentityAdapter) Blocking() bool     { return true }
func (clusterIdentityAdapter) Scope() check.Scope { return check.ScopeOverlay }
func (clusterIdentityAdapter) CheckOverlay(overlayPath, cluster string) []check.Finding {
	// No provider configured -> disabled entirely, not "run against an
	// empty index". clusterid.RawFindings' infraID-mismatch and
	// invalid-JSON structural findings don't depend on the index's
	// contents at all, so calling it with a zero-value ClusterIndex would
	// still produce those findings for any repo that happens to reference
	// an "infraID" key or contain malformed JSON under an overlay - noise
	// for a generic run that never configured cluster-identity checking in
	// the first place.
	if ClusterIndexProvider == nil {
		return nil
	}
	idx := ClusterIndexProvider()
	raw := clusterid.RawFindings(overlayPath, cluster, idx)
	out := make([]check.Finding, 0, len(raw))
	for _, f := range raw {
		// Use the finding's own CheckID (exempt.IDProjectRef/IDClusterName -
		// both exemptable) rather than hardcoding exempt.IDClusterIdentity
		// for everything, which would make every cluster-identity finding
		// permanently non-exemptable regardless of type. Only structural
		// findings that don't set a more specific ID (none currently exist,
		// but a future infraID-mismatch/invalid-JSON check would use this
		// path deliberately, per exempt.IDClusterIdentity's documented
		// fail-closed contract) fall back to the non-exemptable bucket.
		checkID := f.CheckID
		if checkID == "" {
			checkID = exempt.IDClusterIdentity
		}
		out = append(out, check.Finding{
			CheckID:     checkID,
			File:        f.File,
			Path:        f.Field,
			Value:       f.Value,
			Token:       f.Token,
			Kind:        f.Kind,
			Name:        f.Name,
			Namespace:   f.Namespace,
			Annotations: f.Annotations,
			Message:     f.Message,
		})
	}
	return out
}
