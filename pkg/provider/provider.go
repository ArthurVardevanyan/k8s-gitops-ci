package provider

import "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/cluster"

// Branding provides report naming.
type Branding interface {
	ReportMarker() string
	ReportTitle() string
	PipelineHeader() string
	// BinaryName is the invoked executable name used in user-facing
	// "reproduce locally" hints (console log + PR comment). Consumers that
	// ship the tool under a different binary name override this so the hint
	// is copy-pasteable; an empty string falls back to the generic default.
	BinaryName() string
}

// CommentPolicy controls pruning of foreign comments.
type CommentPolicy interface {
	ForeignMarkers() []string
}

// SecretBackend returns context-sensitive auth hints.
type SecretBackend interface {
	AuthErrorHint(stderr string) string
}

// ClusterMetadata returns cluster identity and change-group metadata.
type ClusterMetadata interface {
	ProjectIdentity() (cluster.ProjectIndex, map[string]bool, bool, error)
	ChangeGroups() (map[string]int, bool)
}

// Providers bundles all org-specific behavior. Nil fields use generic defaults.
type Providers struct {
	Branding        Branding
	CommentPolicy   CommentPolicy
	Secrets         SecretBackend
	ClusterMetadata ClusterMetadata
}

const (
	defaultReportMarker   = "<!-- gitops-ci-report -->"
	defaultReportTitle    = "GitOps CI Results"
	defaultPipelineHeader = "GitOps CI Pipeline"
	defaultBinaryName     = "k8s-gitops-ci"
)

// ReportMarker returns the marker string used to identify the unified PR comment.
func (p Providers) ReportMarker() string {
	if p.Branding != nil && p.Branding.ReportMarker() != "" {
		return p.Branding.ReportMarker()
	}
	return defaultReportMarker
}

// ReportTitle returns the report title.
func (p Providers) ReportTitle() string {
	if p.Branding != nil && p.Branding.ReportTitle() != "" {
		return p.Branding.ReportTitle()
	}
	return defaultReportTitle
}

// PipelineHeader returns the pipeline header line.
func (p Providers) PipelineHeader() string {
	if p.Branding != nil && p.Branding.PipelineHeader() != "" {
		return p.Branding.PipelineHeader()
	}
	return defaultPipelineHeader
}

// BinaryName returns the executable name used in "reproduce locally" hints.
func (p Providers) BinaryName() string {
	if p.Branding != nil && p.Branding.BinaryName() != "" {
		return p.Branding.BinaryName()
	}
	return defaultBinaryName
}

// ForeignMarkers returns markers for comments that should be pruned.
func (p Providers) ForeignMarkers() []string {
	if p.CommentPolicy != nil {
		return p.CommentPolicy.ForeignMarkers()
	}
	return nil
}

// SecretAuthHint returns a context-sensitive auth error hint if available.
func (p Providers) SecretAuthHint(stderr string) string {
	if p.Secrets != nil {
		return p.Secrets.AuthErrorHint(stderr)
	}
	return ""
}

// ProjectIdentity returns cluster project identity metadata.
func (p Providers) ProjectIdentity() (idx cluster.ProjectIndex, idToCluster map[string]bool, ok bool, err error) {
	if p.ClusterMetadata != nil {
		return p.ClusterMetadata.ProjectIdentity()
	}
	return cluster.ProjectIndex{}, nil, false, nil
}

// ChangeGroups returns cluster change-group mapping.
func (p Providers) ChangeGroups() (map[string]int, bool) {
	if p.ClusterMetadata != nil {
		return p.ClusterMetadata.ChangeGroups()
	}
	return nil, false
}
