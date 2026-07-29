package validator

import (
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/clusterid"
)

func init() {
	check.Register(docAdapter{id: "namespace", title: "Namespace Scope"})
	check.Register(docAdapter{id: "psa-labels", title: "PSA Namespace Labels"})
	check.Register(docAdapter{id: "rbac-wildcards", title: "RBAC Wildcards"})
	check.Register(docAdapter{id: "rbac-readonly", title: "RBAC Read-Only Aggregate"})
	check.Register(docAdapter{id: "crb", title: "ClusterRoleBinding Subject Namespace"})
	check.Register(docAdapter{id: "sync-options", title: "Argo CD Sync Options"})
	check.Register(docAdapter{id: "image-checksum", title: "Image Digest Pinning"})
	check.Register(docAdapter{id: "named-ports", title: "Named Ports"})
	check.Register(docAdapter{id: "podspec-defaults", title: "PodSpec Defaults"})
	check.Register(docAdapter{id: "placeholder", title: "Unresolved Placeholders"})
	check.Register(clusterIdentityAdapter{})
}

type docAdapter struct {
	id, title string
}

func (d docAdapter) ID() string      { return d.id }
func (d docAdapter) Title() string   { return d.title }
func (d docAdapter) Section() string { return "resource-compliance" }
func (d docAdapter) Blocking() bool  { return true }
func (d docAdapter) Scope() check.Scope {
	return check.ScopeDoc
}
func (d docAdapter) CheckDoc(data []byte, source string) []check.Finding {
	return nil
}

type clusterIdentityAdapter struct{}

func (clusterIdentityAdapter) ID() string      { return "cluster-identity" }
func (clusterIdentityAdapter) Title() string   { return "Cluster Identity Copy/Paste" }
func (clusterIdentityAdapter) Section() string { return "resource-compliance" }
func (clusterIdentityAdapter) Blocking() bool  { return true }
func (clusterIdentityAdapter) Scope() check.Scope {
	return check.ScopeOverlay
}
func (clusterIdentityAdapter) CheckOverlay(overlayPath, cluster string) []check.Finding {
	_ = clusterid.Options{}
	return nil
}
