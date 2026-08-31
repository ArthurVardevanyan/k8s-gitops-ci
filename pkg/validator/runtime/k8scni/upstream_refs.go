package k8scni

import (
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// ovnRepo is the ovn-kubernetes monorepo. Its module path is published from
// the go-controller/ subdirectory (see go.mod), which UpstreamRef.Repo does
// not need to know about - Path below is relative to the repo root, matching
// how a kubernetes/kubernetes ref's Path is relative to that repo's root.
const ovnRepo = "ovn-kubernetes/ovn-kubernetes"

// ovnValidatedAt is the ovn-kubernetes commit every OVN-repo digest below was
// taken at. It is a commit hash, not a release tag: ovn-kubernetes is pinned
// in go.mod by a Go pseudo-version (an untagged commit), which
// `task verify:upstream-refs` resolves to this same trailing hash - see
// pkg/validator/runtime/upstream.go's ValidatedAt doc comment.
const ovnValidatedAt = "e63fce3cf15d"

// upstreamRefs cites the exact upstream function each check in this package
// ports or imports. See pkg/validator/runtime/upstream.go for why a
// file-only citation is not accepted, and docs/CI.md for the standard.
var upstreamRefs = map[string]runtime.UpstreamRef{
	"k8scni/net-attach-def/config-invalid": {
		Repo: "containernetworking/cni",
		Kind: runtime.RefKindImport,
		Path: "libcni/conf.go",
		Functions: []string{
			"NetworkPluginConfFromBytes",
			"ConfListFromBytes", "NetworkConfFromBytes",
		},
		Note: "Calls containernetworking/cni's own reference parser directly (the library every " +
			"CNI-compliant container runtime uses to load a plugin config before invoking it) " +
			"rather than reimplementing the CNI Specification's config-shape rules by hand. " +
			"NetworkPluginConfFromBytes (single-plugin config) rejects a missing \"type\"; " +
			"ConfListFromBytes (a conflist, wrapping NetworkConfFromBytes) additionally " +
			"rejects a missing \"name\", an empty/absent \"plugins\" list, and delegates each entry back " +
			"through NetworkPluginConfFromBytes (via the deprecated ConfFromBytes, an unexported call " +
			"this citation does not separately need since it is a pure forwarding wrapper) so a plugin " +
			"missing \"type\" is rejected there too. Dispatch between " +
			"the two (hasPluginsKey, in probe.go) mirrors the same \"plugins\" key test ovn-kubernetes's " +
			"own pkg/config/cni.go ParseNetConf performs.",
	},
	"k8scni/net-attach-def/ovn-netconf-invalid": {
		Repo:        ovnRepo,
		Kind:        runtime.RefKindRewrite,
		Path:        "go-controller/pkg/util/multi_network.go",
		Functions:   []string{"ValidateNetConf"},
		Digest:      "sha256:addd9b4bebe3a7cc3193c841ce6489cc694253ee06e4f97f81aefff66557e7c0",
		ValidatedAt: ovnValidatedAt,
		Note: "Ports every statically-knowable branch of ValidateNetConf: the netAttachDefName " +
			"consistency check, allowPersistentIPs (topology + subnets), role, ipam.type " +
			"(including the localnet-only dhcp allowance and its subnets exclusivity), " +
			"transport, outboundSNAT, joinSubnet, and the layer2-only subnet/infrastructureSubnets/" +
			"reservedSubnets/defaultGatewayIPs rules. Deliberately not ported: the Uplink field's " +
			"gateway-mode requirement (config.Gateway.Mode == config.GatewayModeShared), which " +
			"depends on live cluster state this tool cannot evaluate from a manifest alone.",
		Additional: []runtime.UpstreamRef{
			{
				Repo:      ovnRepo,
				Kind:      runtime.RefKindImport,
				Path:      "go-controller/pkg/config/cni.go",
				Functions: []string{"ParseNetConf"},
				Note: "Called directly to parse spec.config into the typed NetConf this check judges, " +
					"and to skip non-OVN NADs (ErrorAttachDefNotOvnManaged) exactly as ovn-kubernetes " +
					"itself does - never treating a NAD owned by a different CNI plugin as a failure.",
			},
			{
				Repo:      ovnRepo,
				Kind:      runtime.RefKindImport,
				Path:      "go-controller/pkg/config/cni.go",
				Functions: []string{"ValidateNetConfNameFields"},
				Note:      "Called directly; ValidateNetConf itself calls this before its own rules.",
			},
		},
	},
}
