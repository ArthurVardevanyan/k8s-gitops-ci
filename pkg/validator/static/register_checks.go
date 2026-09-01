package static

import (
	"bytes"
	"strings"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/exempt"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/static/clusterid"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/static/crb"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/static/image"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/static/namedport"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/static/namespace"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/static/placeholder"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/static/podspec"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/static/psa"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/static/rbac"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/static/syncopts"
)

// PsaMissingLabelsExtraKey is the check.Finding.Extra key PSACheck.CheckDoc
// uses to carry a PSA-labels finding's raw, comma-separated MissingLabels,
// so filterCommentedPSAFindings can check each missing label individually
// against psa.FindCommentedNamespaces without re-parsing the rendered Message.
const PsaMissingLabelsExtraKey = "missing_labels"

// ── namespace ────────────────────────────────────────────────────────────────

type NamespaceCheck struct{}

func (NamespaceCheck) ID() string            { return "namespace" }
func (NamespaceCheck) Title() string         { return "Namespace Scope" }
func (NamespaceCheck) Section() string       { return "resource-compliance" }
func (NamespaceCheck) Blocking() bool        { return true }
func (NamespaceCheck) Scope() check.Scope    { return check.ScopeDoc }
func (NamespaceCheck) RenderSensitive() bool { return true }
func (NamespaceCheck) CheckDoc(data []byte, source string) []check.Finding {
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

type PSACheck struct{}

func (PSACheck) ID() string            { return "psa-labels" }
func (PSACheck) Title() string         { return "PSA Namespace Labels" }
func (PSACheck) Section() string       { return "resource-compliance" }
func (PSACheck) Blocking() bool        { return true }
func (PSACheck) Scope() check.Scope    { return check.ScopeDoc }
func (PSACheck) RenderSensitive() bool { return true }
func (PSACheck) CheckDoc(data []byte, source string) []check.Finding {
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
			Extra: map[string]string{PsaMissingLabelsExtraKey: strings.Join(e.MissingLabels, ",")},
		})
	}
	return out
}

// ── rbac-readonly ─────────────────────────────────────────────────────────────

type RBACReadonlyCheck struct{}

func (RBACReadonlyCheck) ID() string            { return "rbac-readonly" }
func (RBACReadonlyCheck) Title() string         { return "RBAC Read-Only Aggregate" }
func (RBACReadonlyCheck) Section() string       { return "resource-compliance" }
func (RBACReadonlyCheck) Blocking() bool        { return true }
func (RBACReadonlyCheck) Scope() check.Scope    { return check.ScopeDoc }
func (RBACReadonlyCheck) RenderSensitive() bool { return true }
func (RBACReadonlyCheck) CheckDoc(data []byte, source string) []check.Finding {
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

type RBACWildcardCheck struct{}

func (RBACWildcardCheck) ID() string            { return "rbac-wildcards" }
func (RBACWildcardCheck) Title() string         { return "RBAC Wildcards" }
func (RBACWildcardCheck) Section() string       { return "resource-compliance" }
func (RBACWildcardCheck) Blocking() bool        { return true }
func (RBACWildcardCheck) Scope() check.Scope    { return check.ScopeDoc }
func (RBACWildcardCheck) RenderSensitive() bool { return true }
func (RBACWildcardCheck) CheckDoc(data []byte, source string) []check.Finding {
	errs := rbac.ValidateWildcardsReader(bytes.NewReader(data), source)
	out := make([]check.Finding, 0, len(errs))
	for _, e := range errs {
		out = append(out, check.Finding{
			CheckID: "rbac-wildcards", File: e.File,
			Kind: e.Kind, Name: e.Resource, Message: e.String(),
			Value:                  e.Field,
			Annotations:            e.Annotations,
			ExemptAnnotationValues: []string{e.Field},
		})
	}
	return out
}

// ── crb ───────────────────────────────────────────────────────────────────────

type CRBCheck struct{}

func (CRBCheck) ID() string            { return "crb" }
func (CRBCheck) Title() string         { return "ClusterRoleBinding Subject Namespace" }
func (CRBCheck) Section() string       { return "resource-compliance" }
func (CRBCheck) Blocking() bool        { return true }
func (CRBCheck) Scope() check.Scope    { return check.ScopeDoc }
func (CRBCheck) RenderSensitive() bool { return true }
func (CRBCheck) CheckDoc(data []byte, source string) []check.Finding {
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

type SyncOptsCheck struct{}

func (SyncOptsCheck) ID() string            { return "sync-options" }
func (SyncOptsCheck) Title() string         { return "Argo CD Sync Options" }
func (SyncOptsCheck) Section() string       { return "resource-compliance" }
func (SyncOptsCheck) Blocking() bool        { return true }
func (SyncOptsCheck) Scope() check.Scope    { return check.ScopeDoc }
func (SyncOptsCheck) RenderSensitive() bool { return true }
func (SyncOptsCheck) CheckDoc(data []byte, source string) []check.Finding {
	errs := syncopts.ValidateReader(bytes.NewReader(data), source)
	out := make([]check.Finding, 0, len(errs))
	for _, e := range errs {
		out = append(out, check.Finding{
			CheckID: "sync-options", File: e.File,
			Kind: e.Kind, Name: e.Name, Namespace: e.Namespace, Message: e.String(),
		})
	}
	return out
}

// ── image-checksum ────────────────────────────────────────────────────────────

type ImageCheck struct{}

func (ImageCheck) ID() string            { return "image-checksum" }
func (ImageCheck) Title() string         { return "Image Digest Pinning" }
func (ImageCheck) Section() string       { return "resource-compliance" }
func (ImageCheck) Blocking() bool        { return true }
func (ImageCheck) Scope() check.Scope    { return check.ScopeDoc }
func (ImageCheck) RenderSensitive() bool { return true }
func (ImageCheck) CheckDoc(data []byte, source string) []check.Finding {
	errs := image.ValidateBytesRaw(data, source)
	out := make([]check.Finding, 0, len(errs))
	for _, e := range errs {
		f := check.Finding{
			CheckID: "image-checksum", File: e.File,
			Kind: e.Kind, Name: e.Name,
			Value:                  e.Image,
			Message:                e.Message,
			Annotations:            e.Annotations,
			ExemptAnnotationValues: []string{e.Image},
		}
		if e.Repo != "" && e.Repo != e.Image {
			f.MatchAliases = []string{e.Repo}
		}
		out = append(out, f)
	}
	return out
}

// ── image-fqdn ────────────────────────────────────────────────────────────────

type ImageFQDNCheck struct{}

func (ImageFQDNCheck) ID() string            { return "image-fqdn" }
func (ImageFQDNCheck) Title() string         { return "Image Registry FQDN" }
func (ImageFQDNCheck) Section() string       { return "resource-compliance" }
func (ImageFQDNCheck) Blocking() bool        { return true }
func (ImageFQDNCheck) Scope() check.Scope    { return check.ScopeDoc }
func (ImageFQDNCheck) RenderSensitive() bool { return true }
func (ImageFQDNCheck) CheckDoc(data []byte, source string) []check.Finding {
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

type NamedPortCheck struct{}

func (NamedPortCheck) ID() string            { return "named-ports" }
func (NamedPortCheck) Title() string         { return "Named Ports" }
func (NamedPortCheck) Section() string       { return "resource-compliance" }
func (NamedPortCheck) Blocking() bool        { return true }
func (NamedPortCheck) Scope() check.Scope    { return check.ScopeDoc }
func (NamedPortCheck) RenderSensitive() bool { return true }
func (NamedPortCheck) CheckDoc(data []byte, source string) []check.Finding {
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

type PodspecCheck struct{}

func (PodspecCheck) ID() string            { return "podspec-defaults" }
func (PodspecCheck) Title() string         { return "PodSpec Defaults" }
func (PodspecCheck) Section() string       { return "resource-compliance" }
func (PodspecCheck) Blocking() bool        { return true }
func (PodspecCheck) Scope() check.Scope    { return check.ScopeDoc }
func (PodspecCheck) RenderSensitive() bool { return true }
func (PodspecCheck) CheckDoc(data []byte, source string) []check.Finding {
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

type PlaceholderCheck struct{}

func (PlaceholderCheck) ID() string            { return "placeholder" }
func (PlaceholderCheck) Title() string         { return "Unresolved Placeholders" }
func (PlaceholderCheck) Section() string       { return "resource-compliance" }
func (PlaceholderCheck) Blocking() bool        { return true }
func (PlaceholderCheck) Scope() check.Scope    { return check.ScopeDoc }
func (PlaceholderCheck) RenderSensitive() bool { return true }
func (PlaceholderCheck) CheckDoc(data []byte, source string) []check.Finding {
	return placeholderFindings(placeholder.ValidateReaderWithOptions(
		bytes.NewReader(data), source, placeholder.Options{CheckAVP: false},
	))
}

func (PlaceholderCheck) CheckRenderedDoc(data []byte, source string) []check.Finding {
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

func (PlaceholderCheck) SkipDoc(kind string) bool { return kind == "CustomResourceDefinition" }

// ── cluster-identity (overlay scope) ─────────────────────────────────────────

type ClusterIdentityAdapter struct{}

func (ClusterIdentityAdapter) ID() string         { return "cluster-identity" }
func (ClusterIdentityAdapter) Title() string      { return "Cluster Identity Copy/Paste" }
func (ClusterIdentityAdapter) Section() string    { return "resource-compliance" }
func (ClusterIdentityAdapter) Blocking() bool     { return true }
func (ClusterIdentityAdapter) Scope() check.Scope { return check.ScopeOverlay }
func (ClusterIdentityAdapter) CheckOverlay(overlayPath, cluster string) []check.Finding {
	if ClusterIndexProvider == nil {
		return nil
	}
	idx := ClusterIndexProvider()
	raw := clusterid.RawFindings(overlayPath, cluster, idx)
	out := make([]check.Finding, 0, len(raw))
	for _, f := range raw {
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
