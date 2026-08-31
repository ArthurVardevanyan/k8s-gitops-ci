package k8scni

import (
	"errors"
	"fmt"
	"strings"

	ovncnitypes "github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/cni/types"
	ovnconfig "github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/config"
	ovntypes "github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/types"
	"sigs.k8s.io/yaml"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// ovnNetConfInvalidCheck applies OVN-Kubernetes' semantic rules to a NAD
// whose CNI type is ovn-k8s-cni-overlay. See package doc comment for why a
// non-OVN NAD is never this check's concern.
type ovnNetConfInvalidCheck struct{ runtime.Meta }

func newOVNNetConfInvalidCheck() ovnNetConfInvalidCheck {
	return ovnNetConfInvalidCheck{runtime.Meta{
		RuleID:    "k8scni/net-attach-def/ovn-netconf-invalid",
		RuleTitle: "OVN-Kubernetes NetworkAttachmentDefinition Semantic Rules",
		AppliesTo: nadKinds,
	}}
}

func (c ovnNetConfInvalidCheck) Run(data []byte, _ string) []runtime.Finding {
	var doc nadDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil
	}

	cfg, err := configString(doc.Spec.Config)
	if err != nil || strings.TrimSpace(cfg) == "" {
		// Malformed/empty spec.config is k8scni/net-attach-def/config-invalid's concern;
		// this check only judges a config that at least parses as a string.
		return nil
	}

	netconf, err := ovnconfig.ParseNetConf([]byte(cfg))
	if err != nil {
		if errors.Is(err, ovnconfig.ErrorAttachDefNotOvnManaged) {
			// Not an OVN NAD. ovn-kubernetes's own ParseNetConf treats this
			// as a no-op skip, never a failure - see the package doc
			// comment - so this check has nothing to say about it.
			return nil
		}
		// Any other parse failure (malformed conflist, chained plugins,
		// ...) is k8scni/net-attach-def/config-invalid's concern too; reporting it again
		// here would double-report the same root cause under a second
		// rule ID.
		return nil
	}

	id := doc.Metadata.Namespace + "/" + doc.Metadata.Name
	if msg, ok := validateOVNNetConf(netconf, id); !ok {
		return []runtime.Finding{runtime.NewFinding(c, check.Finding{
			Kind:      "NetworkAttachmentDefinition",
			Name:      doc.Metadata.Name,
			Namespace: doc.Metadata.Namespace,
			Path:      "spec.config",
			Message:   msg,
		})}
	}
	return nil
}

// validateOVNNetConf applies OVN-Kubernetes' semantic rules to an
// already-parsed OVN netconf. Ported from
// go-controller/pkg/util/multi_network.go's ValidateNetConf - see this
// check's UpstreamRef in upstream_refs.go for the exact citation.
//
// Deliberate divergence: the Uplink field's gateway-mode requirement
// (config.Gateway.Mode == config.GatewayModeShared) is not ported. Gateway
// mode is live cluster state, not something statically knowable from a
// manifest alone - this mirrors every other runtime check's treatment of
// feature-gated or cluster-state-dependent branches (see docs/CI.md's
// version-skew section).
func validateOVNNetConf(netconf *ovncnitypes.NetConf, id string) (msg string, ok bool) {
	fail := func(format string, args ...any) (string, bool) {
		return fmt.Sprintf(format, args...), false
	}

	if netconf.Name != ovntypes.DefaultNetworkName && netconf.NADName != id {
		return fail("netAttachDefName in config (%s) does not match the NAD's namespace/name", netconf.NADName)
	}

	if err := ovnconfig.ValidateNetConfNameFields(netconf); err != nil {
		return fail("%s", err.Error())
	}

	if netconf.AllowPersistentIPs {
		if netconf.Topology == ovntypes.Layer3Topology {
			return fail("layer3 topology does not allow persistent IPs")
		}
		if netconf.Subnets == "" {
			return fail("allowPersistentIPs requires OVN-Kubernetes-managed IPAM (the subnets attribute must be set)")
		}
	}

	if netconf.Role != "" && netconf.Role != ovntypes.NetworkRoleSecondary && netconf.Topology == ovntypes.LocalnetTopology {
		return fail("unexpected network field \"role\" %s for \"localnet\" topology, "+
			"localnet topology does not allow network roles to be set since its always a secondary network", netconf.Role)
	}

	if netconf.Role != "" && netconf.Role != ovntypes.NetworkRolePrimary && netconf.Role != ovntypes.NetworkRoleSecondary {
		return fail("invalid network role value %s", netconf.Role)
	}

	if netconf.IPAM.Type != "" && netconf.IPAM.Type != ovntypes.IPAMTypeDHCP {
		return fail("unsupported ipam key")
	}
	if netconf.IPAM.Type == ovntypes.IPAMTypeDHCP && netconf.Topology != ovntypes.LocalnetTopology {
		return fail("ipam.type %q is only supported with localnet topology", netconf.IPAM.Type)
	}
	if netconf.IPAM.Type == ovntypes.IPAMTypeDHCP && netconf.Subnets != "" {
		return fail("ipam.type %q cannot be used together with the subnets attribute; "+
			"addresses are assigned by the external DHCP server", netconf.IPAM.Type)
	}

	if netconf.Transport != "" &&
		netconf.Transport != ovntypes.NetworkTransportNoOverlay &&
		netconf.Transport != ovntypes.NetworkTransportEVPN {
		return fail("invalid transport %q: must be one of %q", netconf.Transport,
			[]string{ovntypes.NetworkTransportNoOverlay, ovntypes.NetworkTransportEVPN})
	}

	if netconf.OutboundSNAT != "" {
		if netconf.Transport != ovntypes.NetworkTransportNoOverlay {
			return fail("outboundSNAT is only valid when transport is %q", ovntypes.NetworkTransportNoOverlay)
		}
		if netconf.OutboundSNAT != ovntypes.NoOverlaySNATEnabled &&
			netconf.OutboundSNAT != ovntypes.NoOverlaySNATDisabled {
			return fail("invalid outboundSNAT %q: must be one of %q", netconf.OutboundSNAT,
				[]string{ovntypes.NoOverlaySNATEnabled, ovntypes.NoOverlaySNATDisabled})
		}
	}

	if netconf.JoinSubnet != "" && netconf.Topology == ovntypes.LocalnetTopology {
		return fail("localnet topology does not allow specifying join-subnet as services are not supported")
	}

	if netconf.Role == ovntypes.NetworkRolePrimary && netconf.Subnets == "" && netconf.Topology == ovntypes.Layer2Topology {
		return fail("the subnet attribute must be defined for layer2 primary user defined networks")
	}

	if netconf.InfrastructureSubnets != "" && netconf.Topology != ovntypes.Layer2Topology {
		return fail("infrastructureSubnets is only supported for layer2 topology")
	}

	if netconf.ReservedSubnets != "" && netconf.Topology != ovntypes.Layer2Topology {
		return fail("reservedSubnets is only supported for layer2 topology")
	}

	if netconf.DefaultGatewayIPs != "" && netconf.Topology != ovntypes.Layer2Topology {
		return fail("defaultGatewayIPs is only supported for layer2 topology")
	}

	return "", true
}
